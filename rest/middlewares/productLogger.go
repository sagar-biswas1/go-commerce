package middlewares

import (
	"log"
	"net/http"
	"time"
)

// ProductLogger is the product module's own logger, applied only to product
// routes.
//
// It records what the global Logger has no business knowing: which product the
// request addressed and how the module answered. Module-level middleware like
// this is the reason the pipeline is composable rather than one global list.
func (m *Middlewares) ProductLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &recorder{ResponseWriter: w}
		next(rec, r)

		id := r.PathValue("id")
		if id == "" {
			id = "-"
		}

		log.Printf("[product] %s %s id=%s status=%d %s",
			r.Method, r.URL.Path, id, rec.statusOrOK(), time.Since(start).Round(time.Microsecond))
	}
}
