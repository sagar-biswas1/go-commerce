package auth

import (
	"net/http"
	"strings"
	"time"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

const basePath = "/auth"

const (
	// RefreshTokenCookieName is where the refresh token lives.
	RefreshTokenCookieName = "refresh_token"

	// refreshTokenCookiePath scopes the cookie to the routes that need it.
	//
	// A cookie sent on every request is a cookie exposed by every request. Scoped
	// to /auth, the browser withholds it from the entire rest of the API, so a
	// bug in a product handler cannot log it and an XSS payload on a catalogue
	// page cannot reach it. The old value was "/token-refresh", a path this API
	// does not serve -- the cookie was set and then never sent anywhere.
	refreshTokenCookiePath = basePath
)

// sessionContext gathers what the request incidentally says about the client, so
// a session listing has something to show its owner.
//
// It is built here rather than in rest/helpers because a shared helper returning
// one would hand every other handler package a dependency it has no use for.
// Both values are recorded and never trusted: they are attacker-controlled, so
// nothing authorizes on them.
func sessionContext(r *http.Request) *domain.SessionContext {
	return &domain.SessionContext{
		UserAgent: helpers.UserAgent(r),
		IPAddress: helpers.ClientIP(r),
	}
}

// setRefreshTokenCookie stores the refresh token in the browser.
//
// HttpOnly is what keeps it out of reach of JavaScript, which is the whole
// reason the refresh token lives in a cookie while the access token is handed to
// the client in the body: script-readable storage means one XSS is a permanent
// session, and an HttpOnly cookie means the same XSS lasts as long as the access
// token does.
//
// SameSite=Strict is affordable here precisely because the cookie is only used
// by /auth routes the client's own page calls -- there is no cross-site flow to
// break.
func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    token,
		Path:     refreshTokenCookiePath,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearRefreshTokenCookie removes it. Every attribute has to match the cookie
// being replaced -- a browser treats name, path and domain as the identity, so a
// clear with a different path leaves the original in place.
func (h *Handler) clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Path:     refreshTokenCookiePath,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

// refreshTokenFromRequest reads the token from the cookie, falling back to the
// request body's own field for a non-browser client that has no cookie jar.
func refreshTokenFromRequest(r *http.Request, fromBody string) string {
	if cookie, err := r.Cookie(RefreshTokenCookieName); err == nil {
		if value := strings.TrimSpace(cookie.Value); value != "" {
			return value
		}
	}
	return strings.TrimSpace(fromBody)
}

// sessionPayload is what a successful login or refresh returns.
//
// The refresh token is not in it. It travels only in the HttpOnly cookie, so a
// client's own JavaScript can never read it -- putting it in the body as well
// would undo exactly what the cookie is for.
type sessionPayload struct {
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresIn   int          `json:"expiresIn"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	User        *domain.User `json:"user"`
}

func newSessionPayload(user *domain.User, pair *domain.TokenPair) sessionPayload {
	return sessionPayload{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
		// Seconds remaining, which is what an OAuth-shaped client expects and
		// what saves it from having to trust its own clock against ours.
		ExpiresIn: int(time.Until(pair.AccessExpiresAt).Seconds()),
		ExpiresAt: pair.AccessExpiresAt,
		User:      user,
	}
}

// sessionLinks tells a freshly authenticated client where it can go next.
func sessionLinks() response.Links {
	return response.Links{
		"self":             basePath + "/me",
		"refresh":          basePath + "/refresh",
		"logout":           basePath + "/logout",
		"logoutEverywhere": basePath + "/logout-all",
		"sessions":         basePath + "/sessions",
		"products":         "/products",
	}
}
