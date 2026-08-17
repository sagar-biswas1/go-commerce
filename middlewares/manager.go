package middleware

import "net/http"

type Middleware func (http.HandlerFunc) http.HandlerFunc

type Manager struct {
	globalMiddlewares[] Middleware
}

func NewManager () *Manager{
	manager:=Manager{
		globalMiddlewares: make([]Middleware,0),
	}
	return &manager
}

func (mngr *Manager) With(middlewares ...Middleware) Middleware{
	return func (next http.HandlerFunc) http.HandlerFunc{
		n:=next

		for i:=len(middlewares)-1 ;i >=0;i--{
			middleware:=middlewares[i]
			n=middleware(n)

		}
		return n
	}
}

func (mngr *Manager) Use(middlewares ...Middleware) *Manager{
		mngr.globalMiddlewares= append(mngr.globalMiddlewares, middlewares...)
		return mngr
	}