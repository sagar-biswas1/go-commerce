package productHandlers

import (
	product "go-commerce/database"
	"go-commerce/utils"
	"net/http"
)

func GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}

	foundProduct, found := product.ByID(id)
	if !found {
		utils.SendError(w, "Product not found", http.StatusNotFound)
		return
	}

	utils.SendData(w, foundProduct, http.StatusOK)
}
