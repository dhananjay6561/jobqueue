package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// AuthStorer is the store interface used by the auth handler.
type AuthStorer interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*queue.User, error)
	GetUserByEmail(ctx context.Context, email string) (*queue.User, error)
	CreateAPIKeyForUser(ctx context.Context, name string, tier queue.APIKeyTier, userID uuid.UUID) (*queue.APIKey, string, error)
	GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error)
}

// AuthHandler handles user registration and login.
type AuthHandler struct {
	store     AuthStorer
	jwtSecret string
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(s AuthStorer, jwtSecret string) *AuthHandler {
	return &AuthHandler{store: s, jwtSecret: jwtSecret}
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

func (h *AuthHandler) signToken(user *queue.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}
