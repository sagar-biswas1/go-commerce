// Package product is the HTTP transport for the product resource. It reads
// requests, calls the application service, and writes replies -- no rules of its
// own, so a rule cannot be enforced here and skipped elsewhere.
package product

import (
	middlewares "go-commerce/rest/middlewares"
)

// Handler serves the product resource. It reaches for nothing: its service and
// its middleware pipeline are both handed to it at construction, which is what
// lets a test drive it with a fake service and no database.
type Handler struct {
	service            Service
	middlewaresManager *middlewares.Manager
}

// NewHandler wires a product handler to the service and module pipeline it
// should use. A nil pipeline means "no module middleware", not a panic at the
// first request.
func NewHandler(service Service, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		service:            service,
		middlewaresManager: moduleMiddlewares,
	}
}
