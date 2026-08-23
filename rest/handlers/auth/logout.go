package auth

import (
	"net/http"
)

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if tokenStr, err := refreshTokenFromRequest(r); err == nil && tokenStr != "" {
		if _, err := h.jwtHelper.ParseRefreshToken(tokenStr); err == nil {
			_ = h.authStore.RevokeRefreshToken(tokenStr)
		}
	}

	clearRefreshTokenCookie(w)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"logged out"}`))
}
