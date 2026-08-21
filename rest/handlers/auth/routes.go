package auth

import (
	middleware "go-commerce/rest/middlewares"
	"net/http"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.middlewares.ThenWith(h.Login, middleware.RequireJSON))
	mux.HandleFunc("POST /auth/register", h.middlewares.ThenWith(h.Register, middleware.RequireJSON))
	mux.HandleFunc("POST /auth/logout", h.middlewares.Then(h.Logout))
}
