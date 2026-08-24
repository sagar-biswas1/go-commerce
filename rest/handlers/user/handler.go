// Package user is the HTTP transport for the user resource.
package user

import (
	middlewares "go-commerce/rest/middlewares"
)

// Handler serves the user resource. Its service and its middleware pipeline are
// both handed to it at construction, which is what lets a test drive it with a
// fake service and no database. A nil pipeline means "no module middleware", not
// a panic at the first request.
type Handler struct {
	service     Service
	middlewares *middlewares.Manager
}

func NewHandler(service Service, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		service:     service,
		middlewares: moduleMiddlewares,
	}
}
