package product

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		response.Fail(w, r, err)
		return
	}

	// 204 is the conventional reply for a delete with nothing left to return.
	response.NoContent(w)
}
