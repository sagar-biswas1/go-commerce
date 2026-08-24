package auth

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login exchanges credentials for a token pair.
//
// Nothing about the request body is logged. The old handler printed the decoded
// payload, which put every password in plaintext into the server log -- the one
// place they are guaranteed to be kept, backed up, and read by people.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	user, pair, err := h.service.Login(r.Context(), body.Email, body.Password, sessionContext(r))
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	h.setRefreshTokenCookie(w, pair.RefreshToken, pair.RefreshExpiresAt)
	response.Item(w, http.StatusOK, newSessionPayload(user, pair), sessionLinks())
}
