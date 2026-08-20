package product

import "net/http"

// RegisterRoutes mounts the product resource on mux. Each handler runs behind
// the module pipeline injected into NewHandler, which itself sits inside the
// server's global pipeline.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", h.middlewares.Then(h.GetProducts))
	mux.HandleFunc("POST /products", h.middlewares.Then(h.CreateProduct))
	mux.HandleFunc("GET /products/{id}", h.middlewares.Then(h.GetProduct))
	mux.HandleFunc("PATCH /products/{id}", h.middlewares.Then(h.PatchProduct))
	mux.HandleFunc("DELETE /products/{id}", h.middlewares.Then(h.DeleteProduct))
}
