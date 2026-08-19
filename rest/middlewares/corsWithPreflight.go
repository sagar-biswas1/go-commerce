package middleware

import "net/http"

// CorsWithPreflight sets the CORS headers and answers OPTIONS preflight
// requests before they ever reach the router.
func CorsWithPreflight(next http.HandlerFunc) http.HandlerFunc {
	handleAllReq := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}

	return HandleCorsMiddleware(handleAllReq)
}
