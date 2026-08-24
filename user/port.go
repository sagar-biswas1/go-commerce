// Package user is the application service for the user aggregate.
//
// It imports domain and nothing else in this module. Everything it needs from
// the outside is an interface declared here, so the arrows all point inward: the
// repository adapter imports this package to satisfy Repository, the transport
// imports it to call Service, and neither is ever imported back.
package user

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Repository is the storage this service needs.
type Repository interface {
	All(ctx context.Context, page domain.Page, filter *domain.UserFilter) (*domain.PageResult[*domain.User], error)
	ByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	Update(ctx context.Context, id uuid.UUID, apply func(*domain.User)) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	TouchLastLogin(ctx context.Context, id uuid.UUID) error
}

// PasswordHasher is how a plaintext password becomes something storable.
//
// It is a port rather than a direct bcrypt call so the cost can be turned down
// in tests -- bcrypt at production cost makes a test suite that creates users
// slow enough that people stop running it.
type PasswordHasher interface {
	Hash(plaintext string) (string, error)
	Compare(hashed, plaintext string) error
}

// SessionRevoker lets this service end sessions it invalidates.
//
// Changing a role or suspending an account has to reach the sessions already
// issued, or the change does not take effect until the last access token
// happens to expire. The user service does not own refresh tokens, so it asks
// through this one-method port.
type SessionRevoker interface {
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// Service is the driving port: what this aggregate offers the outside world.
//
// Every type in these signatures comes from domain, so the transport can declare
// a port of the same shape without importing this package -- and a handler test
// can drive those routes with a fake instead of a service.
//
// An entity is passed by pointer: it is the largest thing that moves, it moves
// across every layer, and a nil is an honest way to say "no user" on the error
// path. Identity, Page and uuid.UUID stay values -- a pointer there would cost
// an indirection to save nothing and add a nil check to code that cannot fail.
type Service interface {
	List(ctx context.Context, page domain.Page, filter *domain.UserFilter) (*domain.PageResult[*domain.User], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Create(ctx context.Context, input *domain.UserCreateInput) (*domain.User, error)
	Update(ctx context.Context, actor domain.Identity, id uuid.UUID, patch *domain.UserPatch) (*domain.User, error)
	ChangePassword(ctx context.Context, actor domain.Identity, id uuid.UUID, current, next string) error
	Delete(ctx context.Context, id uuid.UUID) error
}
