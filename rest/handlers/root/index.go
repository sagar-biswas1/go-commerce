package root

import (
	"context"
	"net/http"
	"time"

	"go-commerce/rest/response"
)

// Index is the API's entry point: a description of the service and a link to
// everything reachable from it.
//
// It answers only the exact path. The mux pattern "GET /" is a prefix match, so
// without this check every unmatched GET in the API would be answered with the
// index and a 200 -- a client's typo would look like success.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		response.Error(w, http.StatusNotFound, response.CodeNotFound,
			"no resource is served at this path")
		return
	}

	response.Item(w, http.StatusOK, map[string]any{
		"service": h.cfg.ServiceName,
		"version": h.cfg.Version,
	}, response.Links{
		"self":     "/",
		"health":   "/health",
		"ready":    "/ready",
		"products": "/products",
		"users":    "/users",
		"register": "/auth/register",
		"login":    "/auth/login",
		"refresh":  "/auth/refresh",
		"me":       "/auth/me",
		"sessions": "/auth/sessions",
	})
}

// NotFound answers anything the router did not match, in the same shape as
// every other reply.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotFound, response.CodeNotFound,
		"no resource is served at this path")
}

// Health reports that the process is running and answering. It deliberately
// checks nothing else: a liveness probe that fails when the database is briefly
// unreachable causes the orchestrator to restart a process that was working.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response.Item(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": h.cfg.ServiceName,
		"version": h.cfg.Version,
		"uptime":  time.Since(h.startedAt).Round(time.Second).String(),
	}, response.Links{"self": "/health"})
}

// readinessTimeout bounds the dependency check, so a hung database makes the
// probe fail rather than making it hang too.
const readinessTimeout = 2 * time.Second

// Ready reports whether the process can actually serve traffic -- which, unlike
// liveness, does depend on the database. A load balancer reads this to decide
// whether to send requests here.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.ready == nil {
		response.Item(w, http.StatusOK, map[string]any{"status": "ready"},
			response.Links{"self": "/ready"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	if err := h.ready.PingContext(ctx); err != nil {
		response.Error(w, http.StatusServiceUnavailable, response.CodeUnavailable,
			"the database is not reachable")
		return
	}

	response.Item(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"database": "reachable",
	}, response.Links{"self": "/ready"})
}
