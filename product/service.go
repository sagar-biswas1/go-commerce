package product

import (
	"context"
	"sync"

	"go-commerce/domain"

	"github.com/google/uuid"
)

type service struct {
	repo Repository
}

// One service per process is enough -- it holds a repository and no per-request
// state -- so a singleton is resolved once and shared. NewService stays exported
// for tests, which want their own instance with a fake repository.
var (
	once     sync.Once
	instance Service
)

// GetService returns the process-wide product service, building it on the first
// call from the repository it is given.
func GetService(repo Repository) Service {
	once.Do(func() {
		instance = NewService(repo)
	})
	return instance
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, page domain.Page, filter domain.ProductFilter) (domain.PageResult[domain.Product], error) {
	if err := filter.Validate(); err != nil {
		return domain.PageResult[domain.Product]{}, err
	}
	return s.repo.All(ctx, page, filter)
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	return s.repo.ByID(ctx, id)
}

func (s *service) Create(ctx context.Context, p domain.Product) (domain.Product, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return domain.Product{}, err
	}
	return s.repo.Create(ctx, p)
}

// Update applies a partial change.
//
// The patch is validated against the product it will produce, not against the
// fields the client sent, because a rule can span fields the request did not
// mention -- and the merge happens inside the repository's locked read, so what
// gets validated is what actually gets written.
func (s *service) Update(ctx context.Context, id uuid.UUID, patch Patch) (domain.Product, error) {
	if patch.Empty() {
		v := domain.NewValidationError()
		v.Add("body", "no updatable fields were provided")
		return domain.Product{}, v.OrNil()
	}

	// Validate the merged result before taking a write lock, so an obviously
	// bad request is rejected without touching the row at all.
	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}

	preview := current
	patch.Apply(&preview)
	preview.Normalize()
	if err := preview.Validate(); err != nil {
		return domain.Product{}, err
	}

	return s.repo.Update(ctx, id, func(p *domain.Product) {
		patch.Apply(p)
		p.Normalize()
	})
}

// Replace is PUT: every field is stated, and anything the client leaves out is
// cleared rather than kept.
func (s *service) Replace(ctx context.Context, id uuid.UUID, replacement domain.Product) (domain.Product, error) {
	replacement.Normalize()
	if err := replacement.Validate(); err != nil {
		return domain.Product{}, err
	}

	return s.repo.Update(ctx, id, func(p *domain.Product) {
		p.Title = replacement.Title
		p.Price = replacement.Price
		p.ImgUrl = replacement.ImgUrl
		p.Description = replacement.Description
	})
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
