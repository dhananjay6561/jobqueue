package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type userCtxKey struct{}

// UserClaims are the JWT payload fields stored in context after validation.
type UserClaims struct {
	UserID uuid.UUID
	Email  string
}

// JWTAuth returns middleware that validates a Bearer JWT and puts UserClaims in context.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if raw == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"missing or invalid authorization token","data":null}`))
				return
			}

			tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !tok.Valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid or expired token","data":null}`))
				return
			}

			claims, ok := tok.Claims.(jwt.MapClaims)
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"malformed token claims","data":null}`))
				return
			}

			userID, err := uuid.Parse(claims["sub"].(string))
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid token subject","data":null}`))
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey{}, &UserClaims{
				UserID: userID,
				Email:  claims["email"].(string),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the validated UserClaims from context.
func UserFromContext(ctx context.Context) *UserClaims {
	c, _ := ctx.Value(userCtxKey{}).(*UserClaims)
	return c
}
