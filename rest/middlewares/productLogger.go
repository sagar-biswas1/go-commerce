package middlewares

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder remembers the status code on its way out so a middleware can
// report it. http.ResponseWriter does not expose what was already written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// ProductLogger is the product module's own logger, applied only to product
// routes. It records what the global Logger has no business knowing: which
// product the request addressed and how the module answered.
func (m *Middlewares) ProductLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// A handler that never calls WriteHeader has implicitly sent 200.
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(recorder, r)

		id := r.PathValue("id")
		if id == "" {
			id = "-"
		}

		log.Printf("[product] %s %s id=%s status=%d %s",
			r.Method, r.URL.Path, id, recorder.status, time.Since(start))
	}
}
