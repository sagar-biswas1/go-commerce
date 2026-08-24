// Package middlewares holds the composable request wrappers and the manager that
// chains them.
package middlewares

import (
	"context"

	"go-commerce/config"
	"go-commerce/domain"

	"github.com/google/uuid"
)

// TokenValidator is the little that the auth middleware needs: turn a raw
// access token into an identity, or refuse it.
//
// Declaring it here rather than importing the JWT adapter is what keeps the
// transport layer independent of how tokens happen to be signed -- and lets a
// handler test hand this middleware a validator that accepts a fixed string.
type TokenValidator interface {
	ParseAccessToken(token string) (domain.Identity, error)
}

// UserStatusChecker is an optional second opinion on an access token.
//
// An access token is a bearer credential: it is trusted on its signature alone,
// which is what makes it cheap and what makes a revoked one keep working until
// it expires. Wiring this in trades that speed for a lookup per request, so a
// suspended account stops being able to act immediately rather than in fifteen
// minutes. Leave it nil and the tokens stay stateless.
type UserStatusChecker interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type Middlewares struct {
	cfg       *config.Config
	validator TokenValidator
	users     UserStatusChecker
}

// NewMiddleWares takes the config and the token validator the auth middleware
// will use, rather than building either.
func NewMiddleWares(cfg *config.Config, validator TokenValidator) *Middlewares {
	return &Middlewares{cfg: cfg, validator: validator}
}

// WithStatusCheck turns on the per-request account check described on
// UserStatusChecker.
func (m *Middlewares) WithStatusCheck(users UserStatusChecker) *Middlewares {
	m.users = users
	return m
}
