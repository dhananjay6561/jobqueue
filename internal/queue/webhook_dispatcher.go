package queue

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// WebhookStore is the subset of store.DB used by the dispatcher.
type WebhookStore interface {
	ListEnabledWebhooks(ctx context.Context) ([]*Webhook, error)
}

// WebhookDispatcher listens for job events and fires HTTP POST requests
// to all enabled webhooks that are subscribed to each event type.
type WebhookDispatcher struct {
	store  WebhookStore
	client *http.Client
}

// NewWebhookDispatcher creates a dispatcher with a 10-second HTTP timeout.
func NewWebhookDispatcher(store WebhookStore) *WebhookDispatcher {
	return &WebhookDispatcher{
		store: store,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Dispatch sends the event to all matching enabled webhooks.
// It is designed to be called from a goroutine — errors are logged, not returned.
func (d *WebhookDispatcher) Dispatch(ctx context.Context, event Event) {
	hooks, err := d.store.ListEnabledWebhooks(ctx)
	if err != nil {
		log.Error().Err(err).Msg("webhook dispatcher: failed to list webhooks")
		return
	}

	eventStr := string(event.Type)
	payload := WebhookPayload{
		Event:     eventStr,
		JobID:     event.JobID,
		JobType:   event.JobType,
		Timestamp: time.Now().UnixMilli(),
	}
	if p, ok := event.Payload.(map[string]any); ok {
		if errMsg, ok := p["error"].(string); ok {
			payload.Error = errMsg
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("webhook dispatcher: failed to marshal payload")
		return
	}

	for _, hook := range hooks {
		if !containsEvent(hook.Events, eventStr) {
			continue
		}
		go d.deliver(context.WithoutCancel(ctx), hook, body)
	}
}

func (d *WebhookDispatcher) deliver(ctx context.Context, hook *Webhook, body []byte) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Str("url", hook.URL).Msg("webhook: failed to create request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "jobqueue-webhook/1.0")

	if hook.Secret != "" {
		sig := computeHMAC(body, hook.Secret)
		req.Header.Set("X-Webhook-Signature", fmt.Sprintf("sha256=%s", sig))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", hook.URL).Msg("webhook: delivery failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn().
			Str("url", hook.URL).
			Int("status", resp.StatusCode).
			Msg("webhook: endpoint returned error status")
	}
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func containsEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}
