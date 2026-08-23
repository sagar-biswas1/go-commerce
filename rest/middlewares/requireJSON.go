package middlewares

import (
	"mime"
	"net/http"
	"strings"
)

// RequireJSON rejects request bodies that are not JSON. It is a route-level
// middleware: it belongs on the routes that actually read a body (POST, PATCH),
// not on a whole module, so it is attached per route via Manager.ThenWith.
func (m *Middlewares) RequireJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			http.Error(w, `{"error":"Content-Type: application/json is required"}`, http.StatusUnsupportedMediaType)
			return
		}

		// Strip any parameters, so "application/json; charset=utf-8" passes.
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			http.Error(w, `{"error":"Content-Type: application/json is required"}`, http.StatusUnsupportedMediaType)
			return
		}

		next(w, r)
	}
}
