// Package auth is the application service for authentication: registration,
// login, refresh-token rotation, and ending sessions.
package auth

import (
	"context"
	"time"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// UserReader is the slice of user storage this service needs. It is narrower
// than the user service's own Repository on purpose: authentication reads users
// and records logins, and nothing here should be able to delete one.
type UserReader interface {
	ByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	TouchLastLogin(ctx context.Context, id uuid.UUID) error
}

// TokenStore is refresh-token storage.
type TokenStore interface {
	Create(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error)
	ByToken(ctx context.Context, token string) (*domain.RefreshToken, error)
	FamilyOf(ctx context.Context, token string) (uuid.UUID, error)
	// Rotate spends oldToken and issues next in its place, atomically. It
	// reports domain.ErrRefreshTokenReused when the token was already spent.
	Rotate(ctx context.Context, oldToken string, next *domain.RefreshToken) (*domain.RefreshToken, error)
	RevokeByToken(ctx context.Context, token string) error
	RevokeByID(ctx context.Context, id, userID uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) (int64, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ActiveByUser(ctx context.Context, userID uuid.UUID, page domain.Page) (*domain.PageResult[*domain.RefreshToken], error)
	DeleteExpired(ctx context.Context, retention time.Duration) (int64, error)
}

// TokenIssuer mints and checks JWTs. The service depends on this rather than on
// a JWT library, so the signing scheme is one implementation detail behind one
// interface.
type TokenIssuer interface {
	IssueAccessToken(userID uuid.UUID, role string) (string, time.Time, error)
	IssueRefreshToken(userID, familyID uuid.UUID) (string, time.Time, error)
	ParseAccessToken(token string) (domain.Identity, error)
	// ParseRefreshToken returns who the token was minted for. It validates the
	// signature and expiry only; whether the token is still spendable is a
	// question for the store.
	ParseRefreshToken(token string) (uuid.UUID, error)
}

// PasswordHasher is how a plaintext password is stored and checked.
type PasswordHasher interface {
	Hash(plaintext string) (string, error)
	Compare(hashed, plaintext string) error
}

// Service is the driving port: what this aggregate offers the outside world.
//
// Every type in these signatures comes from domain, so the transport can declare
// a port of the same shape without importing this package -- and a handler test
// can drive those routes with a fake instead of a service.
//
// The registration input and the session context live in domain for the same
// reason: both the transport that builds one and this service that consumes one
// have to name it, and neither may import the other.
type Service interface {
	Register(ctx context.Context, input *domain.RegisterInput) (*domain.User, error)
	Login(ctx context.Context, email, password string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error)
	// Refresh rotates a refresh token, returning a fresh pair.
	Refresh(ctx context.Context, refreshToken string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutEverywhere(ctx context.Context, userID uuid.UUID) (int64, error)
	Me(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	Sessions(ctx context.Context, userID uuid.UUID, page domain.Page) (*domain.PageResult[*domain.RefreshToken], error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
}
