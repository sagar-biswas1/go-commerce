package auth

import (
	"net/http"
	"time"

	helpers "go-commerce/rest/helpers"
)

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie != nil {
		if tokenStr := cookie.Value; tokenStr != "" {
			if _, err := helpers.ParseRefreshToken(tokenStr); err == nil {
				_ = h.authStore.RevokeRefreshToken(tokenStr)
			}
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/refresh",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"logged out"}`))
}
