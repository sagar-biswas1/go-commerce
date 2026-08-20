package product

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.productID(w, r)
	if !ok {
		return
	}

	if !h.store.Delete(id) {
		utils.SendError(w, "Product not found", http.StatusNotFound)
		return
	}

	// 204 is the conventional reply for a delete with nothing left to return.
	w.WriteHeader(http.StatusNoContent)
}
