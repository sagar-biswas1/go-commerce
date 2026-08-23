package auth

import (
	"net/http"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.middlewares.ThenWith(h.Login, h.middlewares.RequireJSON))
	mux.HandleFunc("POST /auth/register", h.middlewares.ThenWith(h.Register, h.middlewares.RequireJSON))
	mux.HandleFunc("POST /auth/logout", h.middlewares.Then(h.Logout))
}
