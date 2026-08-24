package middlewares

import (
	"log"
	"net/http"
	"runtime/debug"

	"go-commerce/rest/response"
)

// Recover turns a panic in a handler into a 500 instead of a dead connection.
//
// net/http already recovers a panicking handler, but it closes the connection
// without a reply: the client sees the request fail with no status at all, and
// the stack goes to the server log with no indication of which request produced
// it. Recovering here means one panic answers one request badly rather than
// looking like a network fault.
//
// It has to be the outermost middleware to cover the ones inside it, so it is
// registered first in the global pipeline.
func (m *Middlewares) Recover(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			// ErrAbortHandler is net/http's way of saying "stop, deliberately".
			// Swallowing it would turn an intentional abort into a logged fault.
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			log.Printf("[panic] %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack())

			// If the handler already wrote a status, this is a no-op with a
			// complaint in the log -- there is no way to retract a sent header.
			response.Error(w, http.StatusInternalServerError, response.CodeInternal,
				"something went wrong handling this request")
		}()

		next(w, r)
	}
}
