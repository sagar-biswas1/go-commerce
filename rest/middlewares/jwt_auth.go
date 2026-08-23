package middlewares

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

func (m *Middlewares) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	jwtHelpers := helpers.GetJWTHelper(m.cfg)

	return func(w http.ResponseWriter, r *http.Request) {

		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorization == "" {
			http.Error(w, "Missing authorization token", http.StatusUnauthorized)
			return
		}

		scheme, token, found := strings.Cut(authorization, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := jwtHelpers.ParseAccessToken(token)
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
