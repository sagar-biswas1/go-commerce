package middleware

import "net/http"

type Middleware func(http.HandlerFunc) http.HandlerFunc

type Manager struct {
	globalMiddlewares []Middleware
}

func NewManager() *Manager {
	manager := Manager{
		globalMiddlewares: make([]Middleware, 0),
	}
	return &manager
}

// Use appends middlewares to the global pipeline. The first one registered
// is the outermost, so it sees the request first and the response last.
func (mngr *Manager) Use(middlewares ...Middleware) *Manager {
	mngr.globalMiddlewares = append(mngr.globalMiddlewares, middlewares...)
	return mngr
}

// With builds a one-off pipeline from the given middlewares, ignoring the globals.
func (mngr *Manager) With(middlewares ...Middleware) Middleware {
	return chain(middlewares)
}

// Then runs handler at the end of the global pipeline.
func (mngr *Manager) Then(handler http.HandlerFunc) http.HandlerFunc {
	return chain(mngr.globalMiddlewares)(handler)
}

// ThenHandler is Then for anything implementing http.Handler, such as a *http.ServeMux.
func (mngr *Manager) ThenHandler(handler http.Handler) http.Handler {
	return mngr.Then(handler.ServeHTTP)
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
