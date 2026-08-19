package productHandlers

import (
	product "go-commerce/database"
	"go-commerce/utils"
	"net/http"
)

func DeleteProduct(w http.ResponseWriter, r *http.Request) {

	id, ok := productID(w, r)
	if !ok {
		return
	}

	if !product.Delete(id) {
		utils.SendError(w, "Product not found", http.StatusNotFound)
		return
	}

	// 204 is the conventional reply for a delete with nothing left to return.
	w.WriteHeader(http.StatusNoContent)
}
