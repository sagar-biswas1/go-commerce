// Package product is the application service for the product aggregate: it
// orchestrates domain rules and storage, and knows nothing about HTTP.
package product

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Repository is the storage this service needs, declared here by the consumer
// rather than by whoever implements it.
//
// That is what keeps the dependency pointing inward: repo imports product to
// satisfy this interface, product never imports repo, and a test can hand the
// service a fake without a database anywhere in the picture.
type Repository interface {
	All(ctx context.Context, page domain.Page, filter domain.ProductFilter) (domain.PageResult[domain.Product], error)
	Create(ctx context.Context, p domain.Product) (domain.Product, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// Update applies a change to a locked row, so a read-modify-write cannot
	// lose a concurrent edit.
	Update(ctx context.Context, id uuid.UUID, apply func(*domain.Product)) (domain.Product, error)
}

// Service is what the transport layer depends on. Handlers hold this interface,
// not the struct, so the HTTP tests can drive them without a service at all.
type Service interface {
	List(ctx context.Context, page domain.Page, filter domain.ProductFilter) (domain.PageResult[domain.Product], error)
	Get(ctx context.Context, id uuid.UUID) (domain.Product, error)
	Create(ctx context.Context, p domain.Product) (domain.Product, error)
	Update(ctx context.Context, id uuid.UUID, patch Patch) (domain.Product, error)
	Replace(ctx context.Context, id uuid.UUID, p domain.Product) (domain.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Patch is a partial update. Pointers are what tell "field omitted" apart from
// "field set to its zero value" -- without them, a PATCH that mentions no price
// and one that sets the price to 0 arrive identical.
type Patch struct {
	Title       *string
	Price       *float64
	ImgUrl      *string
	Description *string
}

// Empty reports a patch that asks for nothing, which is a client mistake worth
// naming rather than a successful no-op.
func (p Patch) Empty() bool {
	return p.Title == nil && p.Price == nil && p.ImgUrl == nil && p.Description == nil
}

// Apply writes the present fields onto a product.
func (p Patch) Apply(target *domain.Product) {
	if p.Title != nil {
		target.Title = *p.Title
	}
	if p.Price != nil {
		target.Price = *p.Price
	}
	if p.ImgUrl != nil {
		target.ImgUrl = *p.ImgUrl
	}
	if p.Description != nil {
		target.Description = *p.Description
	}
}
