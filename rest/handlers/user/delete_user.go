package user

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

// DeleteUser soft-deletes a user and ends their sessions. A caller may close
// their own account; closing anyone else's is an admin's privilege.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	if actor := middlewares.MustIdentity(r.Context()); !actor.CanActOn(id) {
		response.Fail(w, r, domain.ErrForbidden)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		response.Fail(w, r, err)
		return
	}

	response.NoContent(w)
}
