package middleware

import "net/http"

func HandleCorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	handleCors:= func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Access-Control-Allow-Origin","*")
        w.Header().Set("Access-Control-Allow-Methods","GET, POST, PUT, PATCH, DELETE, OPTIONS")

		w.Header().Set("Access-Control-Allow-Headers","Content-Type, x-api-key")
		w.Header().Set("Content-Type", "application/json")
		next(w,r)
	}
	
	return handleCors
}
