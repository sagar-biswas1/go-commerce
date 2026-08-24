// Package product is the application service for the product aggregate: it
// orchestrates domain rules and storage, and knows nothing about HTTP.
//
// It imports domain and nothing else in this module. Everything it needs from
// the outside is an interface declared right here, so the arrows all point
// inward: the repository adapter imports this package to satisfy Repository, the
// transport imports it to call Service, and neither of them is ever imported
// back.
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
	All(ctx context.Context, page domain.Page, filter *domain.ProductFilter) (*domain.PageResult[*domain.Product], error)
	Create(ctx context.Context, p *domain.Product) (*domain.Product, error)
	ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// Update applies a change to a locked row, so a read-modify-write cannot
	// lose a concurrent edit. The caller passes what to change rather than the
	// finished row, because the read it is based on has to happen inside the
	// lock to be worth taking.
	Update(ctx context.Context, id uuid.UUID, apply func(*domain.Product)) (*domain.Product, error)
}

// Service is the driving port: what this aggregate offers the outside world.
//
// The transport holds an interface of its own shape rather than this one, and
// both are satisfied by the same implementation because every type in the
// signatures comes from domain. That is deliberate -- it means a handler can be
// driven by a fake in a test without importing this package at all.
type Service interface {
	List(ctx context.Context, page domain.Page, filter *domain.ProductFilter) (*domain.PageResult[*domain.Product], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) (*domain.Product, error)
	Update(ctx context.Context, id uuid.UUID, patch *domain.ProductPatch) (*domain.Product, error)
	Replace(ctx context.Context, id uuid.UUID, p *domain.Product) (*domain.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// A note on what is and is not a pointer here.
//
// An entity is passed by pointer: it is the largest thing that moves, it moves
// across every layer, and a nil is an honest way to say "no product" on the
// error path -- better than a zero Product a caller might read by mistake.
//
// Page, Sort and uuid.UUID stay values. They are two words apiece, so a pointer
// costs an indirection to save nothing, and it would add a nil check to code
// that currently cannot fail. Context stays a value because it is an interface
// already: *context.Context defeats the cancellation it exists to carry.
