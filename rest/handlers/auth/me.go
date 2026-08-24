package auth

import (
	"net/http"

	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

// Me returns the authenticated user.
//
// It reads the record rather than echoing the token's claims, so a client asking
// "who am I" gets the current answer -- a role or status changed since the token
// was minted shows up here.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	actor := middlewares.MustIdentity(r.Context())

	found, err := h.service.Me(r.Context(), actor.UserID)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := sessionLinks()
	links["user"] = response.Path("/users", found.ID.String())
	links["changePassword"] = response.Path("/users", found.ID.String(), "password")

	response.Item(w, http.StatusOK, found, links)
}
