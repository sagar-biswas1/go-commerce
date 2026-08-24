package product

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Service is the application service this handler drives, declared here by the
// consumer that uses it.
//
// It names only domain types, which is what keeps this file honest: the handler
// depends on the vocabulary of the business and on no package that sits beside
// or below it. The real product service satisfies this because Go matches
// interfaces by shape, not by name -- so nothing has to import the handler, and
// a test can drive these routes with a fake that is twenty lines long.
type Service interface {
	List(ctx context.Context, page domain.Page, filter *domain.ProductFilter) (*domain.PageResult[*domain.Product], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) (*domain.Product, error)
	Update(ctx context.Context, id uuid.UUID, patch *domain.ProductPatch) (*domain.Product, error)
	Replace(ctx context.Context, id uuid.UUID, p *domain.Product) (*domain.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
