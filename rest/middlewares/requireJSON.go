package middlewares

import (
	"mime"
	"net/http"
	"strings"

	"go-commerce/rest/response"
)

// RequireJSON rejects a request body that is not JSON.
//
// It is a route-level middleware: it belongs on the routes that actually read a
// body, not on a whole module, so it is attached per route via Manager.ThenWith.
func (m *Middlewares) RequireJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			unsupportedMedia(w)
			return
		}

		// Strip the parameters, so "application/json; charset=utf-8" passes.
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			unsupportedMedia(w)
			return
		}

		next(w, r)
	}
}

func unsupportedMedia(w http.ResponseWriter) {
	// The header names what would be accepted, which is what makes a 415
	// actionable rather than just a refusal.
	w.Header().Set("Accept-Post", "application/json")
	response.Error(w, http.StatusUnsupportedMediaType, response.CodeUnsupportedType,
		"Content-Type must be application/json")
}
