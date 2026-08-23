package middlewares

import (
	"net/http"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

// Manager represents a composable middleware pipeline. It is intentionally
// reusable: one instance can be used as the global server pipeline, while
// another can be configured per module or route.
type Manager struct {
	middlewares *Middlewares
	stack       []Middleware
}

func (m *Middlewares) NewManager(middlewares ...Middleware) *Manager {
	manager := &Manager{
		middlewares: m,
		stack:       make([]Middleware, 0, len(middlewares)),
	}
	manager.Use(middlewares...)
	return manager
}

// Use appends middleware to the current pipeline. The first middleware added is
// the outermost in the chain, so it sees the request first and the response last.
func (mngr *Manager) Use(middlewares ...Middleware) *Manager {
	mngr.stack = append(mngr.stack, middlewares...)
	return mngr
}

// With creates a one-off pipeline from the provided middleware list without
// mutating the current manager.
func (mngr *Manager) With(middlewares ...Middleware) Middleware {
	return chain(middlewares)
}

// Then runs the handler through the manager's pipeline.
func (mngr *Manager) Then(handler http.HandlerFunc) http.HandlerFunc {
	return chain(mngr.stack)(handler)
}

// ThenWith runs the handler through the manager's pipeline and then attaches
// route-specific middleware, which sits closest to the handler.
func (mngr *Manager) ThenWith(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	pipeline := append(append([]Middleware{}, mngr.stack...), middlewares...)
	return chain(pipeline)(handler)
}

// ThenHandler is Then for anything implementing http.Handler, such as a *http.ServeMux.
func (mngr *Manager) ThenHandler(handler http.Handler) http.Handler {
	return mngr.Then(handler.ServeHTTP)
}

func (mngr *Manager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.RequireAuth(next)
}

func (mngr *Manager) RequireJSON(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.RequireJSON(next)
}

func (mngr *Manager) Logger(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.Logger(next)
}

func (mngr *Manager) ProductLogger(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.ProductLogger(next)
}

func (mngr *Manager) CorsWithPreflight(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.CorsWithPreflight(next)
}

func chain(middlewares []Middleware) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		n := next

		for i := len(middlewares) - 1; i >= 0; i-- {
			middleware := middlewares[i]
			n = middleware(n)
		}
		return n
	}
}
