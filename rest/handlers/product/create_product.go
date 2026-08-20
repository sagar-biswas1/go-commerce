package product

import (
	"encoding/json"
	db "go-commerce/database"
	"go-commerce/utils"
	"net/http"
	"strconv"
)

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var newProduct db.Product

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&newProduct); err != nil {
		utils.SendError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if msg, ok := validateTitle(newProduct.Title); !ok {
		utils.SendError(w, msg, http.StatusUnprocessableEntity)
		return
	}
	if msg, ok := validatePrice(newProduct.Price); !ok {
		utils.SendError(w, msg, http.StatusUnprocessableEntity)
		return
	}

	created := h.store.Create(newProduct)
	w.Header().Set("Location", "/products/"+strconv.Itoa(created.ID))
	utils.SendData(w, created, http.StatusCreated)
}
