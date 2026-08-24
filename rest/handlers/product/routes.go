package product

import (
	"net/http"
)

// RegisterRoutes mounts the product resource on mux.
//
// Reads are public; every write is behind RequireAuth and an admin role, because
// the catalogue is the shop's and not its customers'. The role gate is composed
// inside RequireAuth, which is what puts the identity in the context for it to
// read -- the order of these arguments is load-bearing.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	m := h.middlewaresManager

	mux.HandleFunc("GET "+basePath, m.Then(h.GetProducts))
	mux.HandleFunc("GET "+basePath+"/{id}", m.Then(h.GetProduct))

	mux.HandleFunc("POST "+basePath,
		m.ThenWith(h.CreateProduct, m.RequireAuth, m.RequireAdmin, m.RequireJSON))
	mux.HandleFunc("PUT "+basePath+"/{id}",
		m.ThenWith(h.PutProduct, m.RequireAuth, m.RequireAdmin, m.RequireJSON))
	mux.HandleFunc("PATCH "+basePath+"/{id}",
		m.ThenWith(h.PatchProduct, m.RequireAuth, m.RequireAdmin, m.RequireJSON))
	mux.HandleFunc("DELETE "+basePath+"/{id}",
		m.ThenWith(h.DeleteProduct, m.RequireAuth, m.RequireAdmin))
}
