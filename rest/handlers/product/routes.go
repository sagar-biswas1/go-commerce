package product

import (
	"net/http"
)

// RegisterRoutes mounts the product resource on mux. Each handler runs behind
// the module pipeline injected into NewHandler, which itself sits inside the
// server's global pipeline.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", h.middlewaresManager.Then(h.GetProducts))
	mux.HandleFunc("POST /products", h.middlewaresManager.ThenWith(h.CreateProduct, h.middlewaresManager.RequireAuth))
	mux.HandleFunc("GET /products/{id}", h.middlewaresManager.Then(h.GetProduct))
	mux.HandleFunc("PATCH /products/{id}", h.middlewaresManager.ThenWith(h.PatchProduct, h.middlewaresManager.RequireAuth))
	mux.HandleFunc("DELETE /products/{id}", h.middlewaresManager.ThenWith(h.DeleteProduct, h.middlewaresManager.RequireAuth))
}
