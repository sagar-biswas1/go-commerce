package middlewares

import "net/http"

// allowedHeaders is what a browser may send on a cross-origin request.
//
// Authorization is on the list because every authenticated endpoint reads it;
// without it a browser's preflight succeeds and the real request is then
// stripped of its token, which looks like a mysterious 401.
const allowedHeaders = "Content-Type, Authorization, X-Requested-With, x-api-key"

const allowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

func (m *Middlewares) HandleCorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		// How long a browser may cache the preflight, so it does not send one
		// before every request.
		w.Header().Set("Access-Control-Max-Age", "86400")
		// The response varies by these, so a cache must not serve one origin's
		// answer to another.
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")

		// Content-Type is deliberately not set here. It used to be, which meant
		// every 204 claimed to be JSON with no body, and every handler that set
		// its own type was overridden by the outermost middleware. Whoever
		// writes the body says what it is.
		next(w, r)
	}
}
