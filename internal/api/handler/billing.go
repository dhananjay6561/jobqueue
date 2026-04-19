package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/webhook"

	appMiddleware "github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
)

// BillingStorer is the store interface used by the billing handler.
type BillingStorer interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*queue.User, error)
	GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error)
	UpdateStripeCustomerID(ctx context.Context, userID uuid.UUID, customerID string) error
	SetAPIKeyStripeSubscription(ctx context.Context, keyID uuid.UUID, subscriptionID string) error
	UpdateAPIKeyTierBySubscription(ctx context.Context, subscriptionID string, tier queue.APIKeyTier) error
}

// BillingHandler handles Stripe checkout and portal sessions.
type BillingHandler struct {
	store                 BillingStorer
	stripeWebhookSecret   string
	proPriceID            string
	businessPriceID       string
	baseURL               string
}

// NewBillingHandler creates a BillingHandler. stripeSecretKey is set on the
// global stripe client so all calls use it automatically.
func NewBillingHandler(s BillingStorer, stripeSecretKey, webhookSecret, proPriceID, businessPriceID, baseURL string) *BillingHandler {
	stripe.Key = stripeSecretKey
	return &BillingHandler{
		store:               s,
		stripeWebhookSecret: webhookSecret,
		proPriceID:          proPriceID,
		businessPriceID:     businessPriceID,
		baseURL:             baseURL,
	}
}

// CreateCheckout handles POST /portal/checkout.
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	claims := appMiddleware.UserFromContext(r.Context())
	if claims == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		Tier  string `json:"tier"`
		KeyID string `json:"key_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var priceID string
	var tier queue.APIKeyTier
	switch body.Tier {
	case "pro":
		priceID = h.proPriceID
		tier = queue.TierPro
	case "business":
		priceID = h.businessPriceID
		tier = queue.TierBusiness
	default:
		writeError(w, r, http.StatusBadRequest, "tier must be 'pro' or 'business'")
		return
	}

	if priceID == "" {
		writeError(w, r, http.StatusServiceUnavailable, "billing not configured")
		return
	}

	// Resolve key ID: use provided one or fall back to user's first key.
	var keyID string
	if body.KeyID != "" {
		keyID = body.KeyID
	} else {
		keys, err := h.store.GetAPIKeysByUserID(r.Context(), claims.UserID)
		if err != nil || len(keys) == 0 {
			writeError(w, r, http.StatusBadRequest, "no API key found for user")
			return
		}
		keyID = keys[0].ID.String()
	}

	// Ensure user has a Stripe customer.
	user, err := h.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "user not found")
		return
	}

	customerID := user.StripeCustomerID
	if customerID == "" {
		c, err := customer.New(&stripe.CustomerParams{
			Email: stripe.String(user.Email),
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "failed to create stripe customer")
			return
		}
		customerID = c.ID
		_ = h.store.UpdateStripeCustomerID(r.Context(), user.ID, customerID)
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(priceID),
			Quantity: stripe.Int64(1),
		}},
		Metadata: map[string]string{
			"user_id": claims.UserID.String(),
			"key_id":  keyID,
			"tier":    string(tier),
		},
		SuccessURL: stripe.String(h.baseURL + "/billing?success=true"),
		CancelURL:  stripe.String(h.baseURL + "/billing"),
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create checkout session")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]string{"url": sess.URL})
}

// CustomerPortal handles POST /portal/customer-portal.
func (h *BillingHandler) CustomerPortal(w http.ResponseWriter, r *http.Request) {
	claims := appMiddleware.UserFromContext(r.Context())
	if claims == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user.StripeCustomerID == "" {
		writeError(w, r, http.StatusBadRequest, "no billing account found — upgrade first")
		return
	}

	sess, err := session.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(user.StripeCustomerID),
		ReturnURL: stripe.String(h.baseURL + "/billing"),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create portal session")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]string{"url": sess.URL})
}

// StripeWebhook handles POST /webhooks/stripe.
func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "failed to read body")
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.stripeWebhookSecret)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	if event.Type == "checkout.session.completed" {
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		keyIDStr := sess.Metadata["key_id"]
		tierStr := sess.Metadata["tier"]
		subscriptionID := ""
		if sess.Subscription != nil {
			subscriptionID = sess.Subscription.ID
		}

		if keyIDStr != "" && subscriptionID != "" {
			keyID, err := uuid.Parse(keyIDStr)
			if err == nil {
				_ = h.store.SetAPIKeyStripeSubscription(r.Context(), keyID, subscriptionID)
			}
		}

		if subscriptionID != "" && tierStr != "" {
			_ = h.store.UpdateAPIKeyTierBySubscription(r.Context(), subscriptionID, queue.APIKeyTier(tierStr))
		}
	}

	w.WriteHeader(http.StatusOK)
}
