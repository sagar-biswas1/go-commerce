package middlewares

import "net/http"

// CorsWithPreflight sets the CORS headers and answers OPTIONS preflight
// requests before they reach the router.
//
// It answers 204 rather than 200: a preflight has no body, and saying so is what
// stops a client from waiting to read one.
func (m *Middlewares) CorsWithPreflight(next http.HandlerFunc) http.HandlerFunc {
	answerPreflight := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}

	return m.HandleCorsMiddleware(answerPreflight)
}
