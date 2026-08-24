package product

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	found, err := h.service.Get(r.Context(), id)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	response.Item(w, http.StatusOK, found, productLinks(found))
}
