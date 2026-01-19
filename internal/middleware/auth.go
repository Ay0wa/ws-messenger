package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Ay0wa/ws-messenger/internal/auth"
	"github.com/google/uuid"
)

type contextKey string

const userIDKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(userIDKey)
	id, ok := v.(uuid.UUID)
	return id, ok
}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func RequireAuth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			userID, err := authSvc.ParseToken(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return r.URL.Query().Get("token")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
