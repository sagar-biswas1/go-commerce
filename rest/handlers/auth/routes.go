package auth

import (
	"net/http"
)

// RegisterRoutes mounts the auth resource on mux.
//
// The split between the public and authenticated halves is deliberate. Register,
// login and refresh cannot require an access token -- refresh exists precisely
// for when the access token has expired -- so each of those authorizes itself:
// login by credentials, refresh by the refresh token it presents. Everything
// that acts on sessions other than the presented one does require a token.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	m := h.middlewares

	mux.HandleFunc("POST "+basePath+"/register",
		m.ThenWith(h.Register, m.RequireJSON))
	mux.HandleFunc("POST "+basePath+"/login",
		m.ThenWith(h.Login, m.RequireJSON))
	// No RequireJSON: a browser refreshing or logging out sends no body at all,
	// and therefore no Content-Type to check.
	mux.HandleFunc("POST "+basePath+"/refresh", m.Then(h.Refresh))
	mux.HandleFunc("POST "+basePath+"/logout", m.Then(h.Logout))

	mux.HandleFunc("GET "+basePath+"/me",
		m.ThenWith(h.Me, m.RequireAuth))
	mux.HandleFunc("POST "+basePath+"/logout-all",
		m.ThenWith(h.LogoutEverywhere, m.RequireAuth))
	mux.HandleFunc("GET "+basePath+"/sessions",
		m.ThenWith(h.Sessions, m.RequireAuth))
	mux.HandleFunc("DELETE "+basePath+"/sessions/{id}",
		m.ThenWith(h.RevokeSession, m.RequireAuth))
}
