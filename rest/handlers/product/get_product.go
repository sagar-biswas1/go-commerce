package product

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.productID(w, r)
	if !ok {
		return
	}

	found, exists := h.store.ByID(id)
	if !exists {
		utils.SendError(w, "Product not found", http.StatusNotFound)
		return
	}

	utils.SendData(w, found, http.StatusOK)
}
