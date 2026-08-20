package product

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	utils.SendData(w, h.store.All(), http.StatusOK)
}
