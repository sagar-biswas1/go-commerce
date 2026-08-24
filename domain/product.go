package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Price       float64   `json:"price" db:"price"`
	ImgUrl      string    `json:"imageUrl" db:"img_url"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

// Product field limits. They mirror the column types, so a value this package
// accepts is a value the database will take.
const (
	MaxProductTitleLen       = 200
	MaxProductDescriptionLen = 5000
	// NUMERIC(10,2) tops out just under 100 million.
	MaxProductPrice = 99_999_999.99
)

// ErrProductNotFound keeps its own identity so a caller can tell a missing
// product from any other missing thing, while still matching ErrNotFound.
var ErrProductNotFound = wrap("product not found", ErrNotFound)

// Normalize trims the incidental whitespace a client sends, so "  Lamp " and
// "Lamp" are not stored as two different titles.
func (p *Product) Normalize() {
	p.Title = strings.TrimSpace(p.Title)
	p.ImgUrl = strings.TrimSpace(p.ImgUrl)
	p.Description = strings.TrimSpace(p.Description)
}

// Validate reports every rule the product breaks, not just the first.
func (p Product) Validate() error {
	v := NewValidationError()

	switch {
	case strings.TrimSpace(p.Title) == "":
		v.Add("title", "title is required")
	case len(p.Title) > MaxProductTitleLen:
		v.Add("title", "title must be 200 characters or fewer")
	}

	switch {
	case p.Price < 0:
		v.Add("price", "price must not be negative")
	case p.Price > MaxProductPrice:
		v.Add("price", "price is larger than this catalogue supports")
	}

	if len(p.Description) > MaxProductDescriptionLen {
		v.Add("description", "description must be 5000 characters or fewer")
	}

	if p.ImgUrl != "" && !looksLikeURL(p.ImgUrl) {
		v.Add("imageUrl", "imageUrl must be an http or https URL")
	}

	return v.OrNil()
}

// ProductFilter narrows a listing. Every field is optional; a zero value means
// "do not filter on this".
type ProductFilter struct {
	Search   string // matches title or description, case-insensitively
	MinPrice *float64
	MaxPrice *float64
	Sort     Sort
}

// Validate rejects a filter that can never match, rather than letting it run
// and return an empty page the client has to interpret.
func (f ProductFilter) Validate() error {
	v := NewValidationError()

	if f.MinPrice != nil && *f.MinPrice < 0 {
		v.Add("minPrice", "minPrice must not be negative")
	}
	if f.MaxPrice != nil && *f.MaxPrice < 0 {
		v.Add("maxPrice", "maxPrice must not be negative")
	}
	if f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice {
		v.Add("minPrice", "minPrice must not exceed maxPrice")
	}

	return v.OrNil()
}

func looksLikeURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

// ProductPatch is a partial change to a product: every field is optional, and a
// nil field means "leave this alone".
//
// The pointers are what tell "field omitted" apart from "field set to its zero
// value". Without them a request that says nothing about price and one that sets
// price to 0 arrive identical, and the second one silently stops working.
//
// It lives here, beside ProductFilter, because more than one layer has to name
// it: the transport builds one, the service applies one. Putting the shared
// vocabulary in the layer both of them already depend on is what keeps them from
// having to depend on each other.
type ProductPatch struct {
	Title       *string
	Price       *float64
	ImgUrl      *string
	Description *string
}

// Empty reports a patch that asks for nothing, which is a client mistake worth
// naming rather than a successful no-op.
func (p ProductPatch) Empty() bool {
	return p.Title == nil && p.Price == nil && p.ImgUrl == nil && p.Description == nil
}

// Apply writes the present fields onto a product, leaving the absent ones as
// they were.
func (p ProductPatch) Apply(target *Product) {
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
