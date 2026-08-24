package middlewares

import (
	"log"
	"net/http"
	"time"
)

// recorder remembers what a handler wrote on its way out.
//
// http.ResponseWriter is write-only -- there is no way to ask it what status was
// sent -- so a middleware that wants to log the outcome has to observe it in
// passing. Shared by both loggers so they cannot disagree about what happened.
type recorder struct {
	http.ResponseWriter
	status  int
	written int
}

func (rec *recorder) WriteHeader(status int) {
	// Only the first WriteHeader counts, which is also how net/http behaves: a
	// second call is ignored with a warning, and recording it would make the log
	// disagree with the wire.
	if rec.status == 0 {
		rec.status = status
	}
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	// A handler that writes a body without calling WriteHeader has implicitly
	// sent 200.
	if rec.status == 0 {
		rec.status = http.StatusOK
	}

	n, err := rec.ResponseWriter.Write(b)
	rec.written += n
	return n, err
}

// statusOrOK reports what was actually sent, treating a handler that wrote
// nothing at all as the 200 net/http will send for it.
func (rec *recorder) statusOrOK() int {
	if rec.status == 0 {
		return http.StatusOK
	}
	return rec.status
}

// Logger records one line per request: what was asked, what was answered, and
// how long it took.
//
// The status is the part worth having. Timing alone cannot distinguish a fast
// success from a fast rejection, which is exactly the difference you want to see
// when something is wrong.
func (m *Middlewares) Logger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &recorder{ResponseWriter: w}
		next(rec, r)

		log.Printf("%s %s %d %dB %s",
			r.Method, r.URL.RequestURI(), rec.statusOrOK(), rec.written,
			time.Since(start).Round(time.Microsecond))
	}
}
