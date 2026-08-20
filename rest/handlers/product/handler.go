package product

import (
	db "go-commerce/database"
	middleware "go-commerce/rest/middlewares"
)

// Store is what the product handlers need from a data layer, declared here by
// the consumer rather than by the database package. The handlers depend on
// this five-method interface, so any implementation satisfies them: the
// in-memory store today, a SQL-backed one or a test fake later, with no edit
// to a single handler.
type Store interface {
	All() []db.Product
	ByID(id int) (db.Product, bool)
	Create(p db.Product) db.Product
	Update(id int, apply func(*db.Product)) (db.Product, bool)
	Delete(id int) bool
}

// Handler serves the product resource. It reaches for nothing: its store and
// its middleware pipeline are both handed to it at construction.
type Handler struct {
	store       Store
	middlewares *middleware.Manager
}

// NewHandler wires a product handler to the store and module pipeline it should
// use. A nil pipeline means "no module middleware", not a panic at first request.
func NewHandler(store Store, middlewares *middleware.Manager) *Handler {
	if middlewares == nil {
		middlewares = middleware.NewManager()
	}

	return &Handler{
		store:       store,
		middlewares: middlewares,
	}
}
