package auth

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

// refreshRequest is the body a non-browser client sends. A browser sends nothing
// and lets the cookie carry the token.
type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Refresh rotates the refresh token and returns a new pair.
//
// The new refresh token replaces the cookie on the way out, and the old one is
// dead by the time this returns. A client that keeps using the old value gets a
// 401 and, more importantly, trips reuse detection -- which is the mechanism
// that turns a stolen refresh token from a permanent session into a detected
// incident.
//
// The failed path clears the cookie. Leaving a token there that the server has
// already rejected means every subsequent request retries with a credential that
// cannot work.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	// The body is optional here, so a decode failure is not fatal: a browser
	// posting nothing at all is the normal case.
	var body refreshRequest
	if r.ContentLength > 0 {
		if err := helpers.DecodeJSON(w, r, &body); err != nil {
			response.Fail(w, r, err)
			return
		}
	}

	token := refreshTokenFromRequest(r, body.RefreshToken)
	if token == "" {
		h.clearRefreshTokenCookie(w)
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"no refresh token was presented")
		return
	}

	user, pair, err := h.service.Refresh(r.Context(), token, sessionContext(r))
	if err != nil {
		h.clearRefreshTokenCookie(w)
		response.Fail(w, r, err)
		return
	}

	h.setRefreshTokenCookie(w, pair.RefreshToken, pair.RefreshExpiresAt)
	response.Item(w, http.StatusOK, newSessionPayload(user, pair), sessionLinks())
}
