package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

type contextKey string

// UserIDKey is the typed context key user_id is stored under. It is only
// ever populated here, from a cryptographically verified JWT — never from
// request body, query parameters, or custom headers.
const UserIDKey contextKey = "user_id"

// Auth returns middleware that verifies the Supabase-issued JWT on every
// request against the project's JWKS (Supabase signs session tokens with
// ES256, not a shared secret) and stores the authenticated user's id in the
// request context. jwks should be created once at startup — it keeps its
// own HTTP client and background refresh goroutine.
func Auth(jwks keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := extractBearerToken(r)
			if !ok {
				apierror.WriteError(w, apierror.Unauthorized(""))
				return
			}

			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(tokenString, claims, jwks.Keyfunc,
				jwt.WithValidMethods([]string{"ES256"}),
				jwt.WithAudience("authenticated"),
			)
			if err != nil {
				slog.Warn("auth: token rejected", "reason", err)
				apierror.WriteError(w, apierror.Unauthorized(""))
				return
			}

			sub, err := claims.GetSubject()
			if err != nil || sub == "" {
				slog.Warn("auth: token missing subject claim")
				apierror.WriteError(w, apierror.Unauthorized(""))
				return
			}

			userID, err := uuid.Parse(sub)
			if err != nil {
				slog.Warn("auth: subject claim is not a valid uuid")
				apierror.WriteError(w, apierror.Unauthorized(""))
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}

// GetUserID returns the authenticated user's id from context. It only
// succeeds for requests that passed through Auth.
func GetUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.UUID{}, errors.New("middleware: user_id not present in context")
	}
	return userID, nil
}
