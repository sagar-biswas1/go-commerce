package product

import (
	"go-commerce/repo"
	middlewares "go-commerce/rest/middlewares"
)

// Store is what the product handlers need from a data layer, declared here by
// the consumer rather than by the database package. The handlers depend on
// this five-method interface, so any implementation satisfies them: the
// in-memory store today, a SQL-backed one or a test fake later, with no edit
// to a single handler.

// Handler serves the product resource. It reaches for nothing: its store and
// its middleware pipeline are both handed to it at construction.
type Handler struct {
	store              repo.ProductRepo
	middlewaresManager *middlewares.Manager
}

// NewHandler wires a product handler to the store and module pipeline it should
// use. A nil pipeline means "no module middleware", not a panic at first request.
func NewHandler(store repo.ProductRepo, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		store:              store,
		middlewaresManager: moduleMiddlewares,
	}
}
