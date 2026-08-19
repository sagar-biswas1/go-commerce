package productHandlers

import (
	product "go-commerce/database"
	"go-commerce/utils"
	"net/http"
)

func GetProducts(w http.ResponseWriter, r *http.Request) {
	utils.SendData(w, product.All(), http.StatusOK)
}
