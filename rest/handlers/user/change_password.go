package user

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword replaces a user's password.
//
// It is its own route rather than a field on PATCH /users/{id} because it is not
// a profile edit: it requires the current password, and it ends every existing
// session. Folding it into the general patch would make both of those conditional
// on which fields happened to be present.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	var body changePasswordRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	actor := middlewares.MustIdentity(r.Context())
	if err := h.service.ChangePassword(r.Context(), actor, id, body.CurrentPassword, body.NewPassword); err != nil {
		response.Fail(w, r, err)
		return
	}

	// Say plainly that the sessions are gone, since the client's own refresh
	// token is now among them and its next refresh will fail.
	response.Item(w, http.StatusOK, map[string]string{
		"message": "password changed; all sessions have been signed out",
	}, response.Links{
		"login": "/auth/login",
		"user":  response.Path(basePath, id.String()),
	})
}
