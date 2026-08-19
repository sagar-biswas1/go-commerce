package productHandlers

import "net/http"

// RegisterRoutes mounts the product resource.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", GetProducts)
	mux.HandleFunc("POST /products", CreateProduct)
	mux.HandleFunc("GET /products/{id}", GetProduct)
	mux.HandleFunc("PATCH /products/{id}", PatchProduct)
	mux.HandleFunc("DELETE /products/{id}", DeleteProduct)
}
