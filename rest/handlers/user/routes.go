package user

import (
	"net/http"
)

// RegisterRoutes mounts the user resource on mux. Each handler runs behind the
// module pipeline injected into NewHandler, which itself sits inside the
// server's global pipeline. Routes that read a body add RequireJSON on top of
// that, which is middleware at the level of a single route.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users", h.middlewares.Then(h.GetUsers))
	// mux.HandleFunc("POST /users", h.middlewares.ThenWith(h.CreateUser, h.middlewares.RequireAuth, h.middlewares.RequireJSON))
	mux.HandleFunc("GET /users/{id}", h.middlewares.Then(h.GetUserById))
	mux.HandleFunc("PATCH /users/{id}", h.middlewares.ThenWith(h.PatchUser, h.middlewares.RequireAuth, h.middlewares.RequireJSON))
	mux.HandleFunc("DELETE /users/{id}", h.middlewares.ThenWith(h.DeleteUser, h.middlewares.RequireAuth))
}
