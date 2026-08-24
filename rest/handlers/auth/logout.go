package auth

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

// Logout ends the session the presented refresh token belongs to.
//
// It needs no access token: logging out is something a client should be able to
// do when its access token has already expired, which is exactly when it most
// wants to. The refresh token it presents is the authorization.
//
// It always reports success. Logging out of a session that is already gone is
// the outcome the caller wanted, and reporting "no such token" would tell an
// unauthenticated caller which tokens exist.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if r.ContentLength > 0 {
		// A malformed body should not stop a logout; the cookie is the usual
		// source anyway.
		_ = helpers.DecodeJSON(w, r, &body)
	}

	if token := refreshTokenFromRequest(r, body.RefreshToken); token != "" {
		if err := h.service.Logout(r.Context(), token); err != nil {
			response.Fail(w, r, err)
			return
		}
	}

	h.clearRefreshTokenCookie(w)
	response.Item(w, http.StatusOK, map[string]string{"message": "signed out"}, response.Links{
		"login":    basePath + "/login",
		"register": basePath + "/register",
	})
}

// LogoutEverywhere ends every session this user has.
//
// Unlike Logout it does require an access token, because it acts on sessions
// other than the one presenting the request -- the caller has to prove who they
// are, not merely hold one of that user's refresh tokens.
func (h *Handler) LogoutEverywhere(w http.ResponseWriter, r *http.Request) {
	actor := middlewares.MustIdentity(r.Context())

	revoked, err := h.service.LogoutEverywhere(r.Context(), actor.UserID)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	h.clearRefreshTokenCookie(w)
	response.Item(w, http.StatusOK, map[string]any{
		"message":         "signed out everywhere",
		"sessionsRevoked": revoked,
	}, response.Links{
		"login": basePath + "/login",
	})
}
