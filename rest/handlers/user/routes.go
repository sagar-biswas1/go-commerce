package user

import (
	"net/http"
)

// RegisterRoutes mounts the user resource on mux.
//
// Nothing here is public. Listing users is admin-only; the single-resource routes
// are open to any authenticated caller and then narrowed to "yourself, or an
// admin" by the handler or the service, which is where the rule can see what is
// actually being asked for.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	m := h.middlewares

	mux.HandleFunc("GET "+basePath,
		m.ThenWith(h.GetUsers, m.RequireAuth, m.RequireAdmin))
	mux.HandleFunc("POST "+basePath,
		m.ThenWith(h.CreateUser, m.RequireAuth, m.RequireAdmin, m.RequireJSON))

	mux.HandleFunc("GET "+basePath+"/{id}",
		m.ThenWith(h.GetUserById, m.RequireAuth))
	mux.HandleFunc("PATCH "+basePath+"/{id}",
		m.ThenWith(h.PatchUser, m.RequireAuth, m.RequireJSON))
	mux.HandleFunc("DELETE "+basePath+"/{id}",
		m.ThenWith(h.DeleteUser, m.RequireAuth))

	mux.HandleFunc("PUT "+basePath+"/{id}/password",
		m.ThenWith(h.ChangePassword, m.RequireAuth, m.RequireJSON))
}
