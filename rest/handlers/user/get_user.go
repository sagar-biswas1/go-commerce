package user

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

// GetUserById returns one user.
//
// A caller may read their own record; reading anyone else's is an admin's
// privilege. The check is here rather than in the service because reading is the
// one operation where the rule is purely about who is asking, with nothing about
// the request body to weigh.
func (h *Handler) GetUserById(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	actor := middlewares.MustIdentity(r.Context())
	if !actor.CanActOn(id) {
		response.Fail(w, r, domain.ErrForbidden)
		return
	}

	found, err := h.service.Get(r.Context(), id)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	response.Item(w, http.StatusOK, found, userLinks(found))
}
