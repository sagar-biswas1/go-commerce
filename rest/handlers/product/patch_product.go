package product

import (
	"encoding/json"
	db "go-commerce/database"
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) PatchProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.productID(w, r)
	if !ok {
		return
	}

	// Pointers let us tell "field omitted" apart from "field set to zero".
	var updates struct {
		Title       *string  `json:"title"`
		Price       *float64 `json:"price"`
		ImgUrl      *string  `json:"imageUrl"`
		Description *string  `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.SendError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if updates.Title != nil {
		if msg, valid := validateTitle(*updates.Title); !valid {
			utils.SendError(w, msg, http.StatusUnprocessableEntity)
			return
		}
	}
	if updates.Price != nil {
		if msg, valid := validatePrice(*updates.Price); !valid {
			utils.SendError(w, msg, http.StatusUnprocessableEntity)
			return
		}
	}

	updated, found := h.store.Update(id, func(p *db.Product) {
		if updates.Title != nil {
			p.Title = *updates.Title
		}
		if updates.Price != nil {
			p.Price = *updates.Price
		}
		if updates.ImgUrl != nil {
			p.ImgUrl = *updates.ImgUrl
		}
		if updates.Description != nil {
			p.Description = *updates.Description
		}
	})

	if !found {
		utils.SendError(w, "Product not found", http.StatusNotFound)
		return
	}

	utils.SendData(w, updated, http.StatusOK)
}
