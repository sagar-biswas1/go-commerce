package user

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/response"
)

// GetUsers lists users, one page at a time. It is admin-only: a directory of
// every account with its email address is not something a customer should be
// able to page through.
func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	page := response.PageFromRequest(r)

	filter := &domain.UserFilter{
		Search: r.URL.Query().Get("q"),
		Role:   r.URL.Query().Get("role"),
		Status: r.URL.Query().Get("status"),
		Sort:   response.SortFromRequest(r),
	}

	result, err := h.service.List(r.Context(), page, filter)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := response.PageLinks(r, result, page)
	links["create"] = basePath

	response.Collection(w, http.StatusOK,
		response.Collect(result.Items, userLinks),
		links,
		response.PageMeta(result, page, result.Total),
	)
}
