package product

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

// PutProduct replaces a product wholesale.
//
// PUT and PATCH differ in what an omitted field means, and that difference is
// the whole reason both exist: PATCH leaves it alone, PUT clears it. A client
// that wants to be sure of the resulting state uses this one.
func (h *Handler) PutProduct(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	var body createRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	replaced, err := h.service.Replace(r.Context(), id, domain.Product{
		Title:       body.Title,
		Price:       body.Price,
		ImgUrl:      body.ImgUrl,
		Description: body.Description,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	response.Item(w, http.StatusOK, replaced, productLinks(replaced))
}
