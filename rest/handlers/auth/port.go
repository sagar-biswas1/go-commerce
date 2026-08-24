package auth

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Service is the application service this handler drives, declared here by the
// consumer that uses it.
//
// As in the other handler packages, every type in these signatures comes from
// domain: the transport names the business vocabulary and never the aggregate
// that implements it, so the dependency runs one way and a test can drive these
// routes without an issuer, a hasher or a database.
type Service interface {
	Register(ctx context.Context, input *domain.RegisterInput) (*domain.User, error)
	Login(ctx context.Context, email, password string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutEverywhere(ctx context.Context, userID uuid.UUID) (int64, error)
	Me(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	Sessions(ctx context.Context, userID uuid.UUID, page domain.Page) (*domain.PageResult[*domain.RefreshToken], error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
}
