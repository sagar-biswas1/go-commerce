package product

import (
	"net/http"

	productsvc "go-commerce/product"
	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

// patchRequest is a partial update. The pointers are what tell "field omitted"
// apart from "field set to its zero value": without them, a body that says
// nothing about price and one that sets price to 0 arrive identical.
type patchRequest struct {
	Title       *string  `json:"title"`
	Price       *float64 `json:"price"`
	ImgUrl      *string  `json:"imageUrl"`
	Description *string  `json:"description"`
}

func (h *Handler) PatchProduct(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	var body patchRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	updated, err := h.service.Update(r.Context(), id, productsvc.Patch{
		Title:       body.Title,
		Price:       body.Price,
		ImgUrl:      body.ImgUrl,
		Description: body.Description,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	response.Item(w, http.StatusOK, updated, productLinks(updated))
}
