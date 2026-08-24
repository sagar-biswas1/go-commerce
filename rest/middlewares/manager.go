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
	if mngr == nil {
		return nil
	}
	mngr.stack = append(mngr.stack, middlewares...)
	return mngr
}

// With creates a one-off pipeline from the provided middleware list without
// mutating the current manager.
func (mngr *Manager) With(middlewares ...Middleware) Middleware {
	return chain(middlewares)
}

// The four composers below tolerate a nil manager, because NewHandler promises a
// module may be built without a pipeline. Without these guards that promise
// fails at route registration -- before a single request arrives, and nowhere
// near the constructor that made the claim.

// Then runs the handler through the manager's pipeline.
func (mngr *Manager) Then(handler http.HandlerFunc) http.HandlerFunc {
	if mngr == nil {
		return handler
	}
	return chain(mngr.stack)(handler)
}

// ThenWith runs the handler through the manager's pipeline and then attaches
// route-specific middleware, which sits closest to the handler.
func (mngr *Manager) ThenWith(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	var stack []Middleware
	if mngr != nil {
		stack = mngr.stack
	}
	pipeline := append(append([]Middleware{}, stack...), middlewares...)
	return chain(pipeline)(handler)
}

// ThenHandler is Then for anything implementing http.Handler, such as a *http.ServeMux.
func (mngr *Manager) ThenHandler(handler http.Handler) http.Handler {
	return mngr.Then(handler.ServeHTTP)
}

// Recover, RequireAuth and the rest are forwarded so a module can reach the
// shared middleware through the manager it was handed, without also being handed
// the Middlewares value. A nil manager returns the handler unwrapped rather than
// panicking on the first request.
func (mngr *Manager) Recover(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.Recover(next)
}

// RequireRole builds a role gate. It must be composed inside RequireAuth, which
// is what puts an identity in the context for it to read.
func (mngr *Manager) RequireRole(roles ...string) Middleware {
	if mngr == nil || mngr.middlewares == nil {
		return func(next http.HandlerFunc) http.HandlerFunc { return next }
	}
	return mngr.middlewares.RequireRole(roles...)
}

func (mngr *Manager) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	if mngr == nil || mngr.middlewares == nil {
		return next
	}
	return mngr.middlewares.RequireAdmin(next)
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
