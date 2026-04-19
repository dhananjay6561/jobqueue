package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	appMiddleware "github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// AuthStorer is the store interface used by the auth handler.
type AuthStorer interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*queue.User, error)
	GetUserByEmail(ctx context.Context, email string) (*queue.User, error)
	CreateAPIKeyForUser(ctx context.Context, name string, tier queue.APIKeyTier, userID uuid.UUID) (*queue.APIKey, string, error)
	GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error)
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
	CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (*store.PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, tokenHash string) error
	RegenerateAPIKey(ctx context.Context, userID uuid.UUID) (*queue.APIKey, string, error)
}

// AuthHandler handles user registration and login.
type AuthHandler struct {
	store     AuthStorer
	jwtSecret string
	baseURL   string
	smtpHost  string
	smtpPort  string
	smtpUser  string
	smtpPass  string
	smtpFrom  string
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(s AuthStorer, jwtSecret, baseURL, smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom string) *AuthHandler {
	return &AuthHandler{
		store:     s,
		jwtSecret: jwtSecret,
		baseURL:   baseURL,
		smtpHost:  smtpHost,
		smtpPort:  smtpPort,
		smtpUser:  smtpUser,
		smtpPass:  smtpPass,
		smtpFrom:  smtpFrom,
	}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" || body.Password == "" {
		writeError(w, r, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(body.Password) < 8 {
		writeError(w, r, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.store.CreateUser(r.Context(), body.Email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, r, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}

	apiKey, rawKey, err := h.store.CreateAPIKeyForUser(r.Context(), "default", queue.TierFree, user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create API key")
		return
	}

	token, err := h.signToken(user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to sign token")
		return
	}

	writeJSON(w, r, http.StatusCreated, map[string]any{
		"token": token,
		"user":  user,
		"api_key": map[string]any{
			"id":         apiKey.ID,
			"key":        rawKey,
			"key_prefix": apiKey.KeyPrefix,
			"tier":       apiKey.Tier,
			"jobs_limit": apiKey.JobsLimit,
			"warning":    "Save this key — it will not be shown again.",
		},
	})
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))

	user, err := h.store.GetUserByEmail(r.Context(), body.Email)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "login failed")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.signToken(user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to sign token")
		return
	}

	keys, _ := h.store.GetAPIKeysByUserID(r.Context(), user.ID)

	writeJSON(w, r, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
		"keys":  keys,
	})
}

// ForgotPassword handles POST /auth/forgot-password.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))

	// Always return the same response to prevent email enumeration.
	generic := map[string]string{"message": "If that email is registered you will receive a reset link shortly."}

	user, err := h.store.GetUserByEmail(r.Context(), body.Email)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, r, http.StatusOK, generic)
		return
	}
	if err != nil {
		writeJSON(w, r, http.StatusOK, generic)
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeJSON(w, r, http.StatusOK, generic)
		return
	}
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	if err := h.store.CreatePasswordResetToken(r.Context(), user.ID, tokenHash); err != nil {
		writeJSON(w, r, http.StatusOK, generic)
		return
	}

	resetURL := fmt.Sprintf("%s/auth/reset?token=%s", h.baseURL, token)
	go h.sendResetEmail(user.Email, resetURL)

	writeJSON(w, r, http.StatusOK, generic)
}

// ResetPassword handles POST /auth/reset-password.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Password) < 8 {
		writeError(w, r, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash := sha256.Sum256([]byte(body.Token))
	tokenHash := hex.EncodeToString(hash[:])

	t, err := h.store.GetPasswordResetToken(r.Context(), tokenHash)
	if errors.Is(err, store.ErrNotFound) || t == nil {
		writeError(w, r, http.StatusBadRequest, "invalid or expired reset token")
		return
	}
	if t.Used {
		writeError(w, r, http.StatusBadRequest, "reset token already used")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := h.store.UpdatePasswordHash(r.Context(), t.UserID, string(newHash)); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to update password")
		return
	}
	_ = h.store.MarkResetTokenUsed(r.Context(), tokenHash)

	writeJSON(w, r, http.StatusOK, map[string]string{"message": "Password updated successfully. You can now sign in."})
}

// RegenerateKey handles POST /portal/regenerate-key.
func (h *AuthHandler) RegenerateKey(w http.ResponseWriter, r *http.Request) {
	claims := appMiddleware.UserFromContext(r.Context())
	if claims == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication required")
		return
	}
	key, rawKey, err := h.store.RegenerateAPIKey(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to regenerate key")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"key":        rawKey,
		"key_prefix": key.KeyPrefix,
		"tier":       key.Tier,
		"jobs_limit": key.JobsLimit,
		"warning":    "Save this key — it will not be shown again.",
	})
}

func (h *AuthHandler) sendResetEmail(to, resetURL string) {
	if h.smtpHost == "" {
		// No SMTP configured — log the reset URL so dev can use it.
		fmt.Printf("[PASSWORD RESET] %s → %s\n", to, resetURL)
		return
	}
	body := fmt.Sprintf("Subject: Reset your JobQueue password\r\n"+
		"From: %s\r\nTo: %s\r\nContent-Type: text/plain\r\n\r\n"+
		"Click the link below to reset your password (expires in 1 hour):\r\n\r\n%s\r\n\r\n"+
		"If you did not request this, ignore this email.\r\n",
		h.smtpFrom, to, resetURL)

	addr := h.smtpHost + ":" + h.smtpPort
	var auth smtp.Auth
	if h.smtpUser != "" {
		auth = smtp.PlainAuth("", h.smtpUser, h.smtpPass, h.smtpHost)
	}
	_ = smtp.SendMail(addr, auth, h.smtpFrom, []string{to}, []byte(body))
}

// GetUsage handles GET /portal/usage — returns usage for the user's primary API key.
func (h *AuthHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	claims := appMiddleware.UserFromContext(r.Context())
	if claims == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication required")
		return
	}
	keys, err := h.store.GetAPIKeysByUserID(r.Context(), claims.UserID)
	if err != nil || len(keys) == 0 {
		writeJSON(w, r, http.StatusOK, map[string]any{
			"tier": "free", "jobs_used": 0, "jobs_limit": 1000,
			"usage_percent": 0, "limit_reached": false,
		})
		return
	}
	k := keys[0]
	writeJSON(w, r, http.StatusOK, map[string]any{
		"tier":          k.Tier,
		"jobs_used":     k.JobsUsed,
		"jobs_limit":    k.JobsLimit,
		"usage_percent": k.UsagePercent(),
		"limit_reached": k.LimitReached(),
		"reset_at":      k.ResetAt,
		"key_id":        k.ID,
		"key_prefix":    k.KeyPrefix,
	})
}

func (h *AuthHandler) signToken(user *queue.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}
