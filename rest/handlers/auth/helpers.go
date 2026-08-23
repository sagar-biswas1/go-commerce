package auth

import (
	"net/http"
	"strings"
	"time"
)

const (
	RefreshTokenCookieName = "refresh_token"
	RefreshTokenCookiePath = "/token-refresh"
)

func setRefreshTokenCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    token,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     RefreshTokenCookiePath,
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Path:     RefreshTokenCookiePath,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

func refreshTokenFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(RefreshTokenCookieName)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cookie.Value) == "" {
		return "", http.ErrNoCookie
	}
	return cookie.Value, nil
}
