package middleware

import (
	"context"
	"net/http"
	"strings"

	helpers "go-commerce/rest/helpers"
)

type contextKey string

const userContextKey contextKey = "user"

type AuthenticatedUser struct {
	UserID int
	Role   string
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			http.Error(w, "Missing authorization token", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := helpers.ParseAccessToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, AuthenticatedUser{
			UserID: claims.UserID,
			Role:   claims.Role,
		})
		next(w, r.WithContext(ctx))
	}
}

func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(userContextKey).(AuthenticatedUser)
	return user, ok
}
