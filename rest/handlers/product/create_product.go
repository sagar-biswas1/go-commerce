package product

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

// createRequest is the accepted shape of a create body.
//
// It is a separate type from domain.Product on purpose: decoding straight into
// the entity would let a request set id, createdAt or updatedAt, which are the
// server's to decide. This type has no fields for them, so there is nothing to
// ignore and nothing to forget to ignore.
type createRequest struct {
	Title       string  `json:"title"`
	Price       float64 `json:"price"`
	ImgUrl      string  `json:"imageUrl"`
	Description string  `json:"description"`
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var body createRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	created, err := h.service.Create(r.Context(), domain.Product{
		Title:       body.Title,
		Price:       body.Price,
		ImgUrl:      body.ImgUrl,
		Description: body.Description,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := productLinks(created)
	// Location is where the new resource lives, which a 201 is required to say.
	w.Header().Set("Location", links["self"])
	response.Item(w, http.StatusCreated, created, links)
}
