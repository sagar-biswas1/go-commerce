// Package root serves the API's entry point and its health check.
//
// The entry point is what makes the rest of the API discoverable: a client that
// knows one URL can find every collection from here, rather than having a list
// of paths compiled into it that goes stale the moment one is renamed.
package root

import (
	"context"
	"net/http"
	"time"

	"go-commerce/config"
	middlewares "go-commerce/rest/middlewares"
)

type Handler struct {
	cfg         *config.Config
	middlewares *middlewares.Manager
	ready       Readiness
	startedAt   time.Time
}

// Readiness is whatever can say the process's dependencies are reachable.
// *sqlx.DB satisfies it, which is what makes the readiness check honest rather
// than a constant 200.
type Readiness interface {
	PingContext(ctx context.Context) error
}

// NewHandler takes the config it reports and the dependency it checks.
func NewHandler(cfg *config.Config, ready Readiness, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		cfg:         cfg,
		middlewares: moduleMiddlewares,
		ready:       ready,
		startedAt:   time.Now().UTC(),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	m := h.middlewares

	mux.HandleFunc("GET /", m.Then(h.Index))
	mux.HandleFunc("GET /health", m.Then(h.Health))
	mux.HandleFunc("GET /ready", m.Then(h.Ready))

	// A pattern with no method matches every method, and "GET /" is more
	// specific so it still wins for GET. Without this, an unmatched POST falls
	// through to net/http's own handler, which answers with plain text -- the one
	// reply in the API that is not JSON, and the one a client hits when it has a
	// typo and most needs a parseable answer.
	mux.HandleFunc("/", m.Then(h.NotFound))
}
