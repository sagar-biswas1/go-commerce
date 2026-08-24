// Package user is the application service for the user aggregate.
package user

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Repository is the storage this service needs.
type Repository interface {
	All(ctx context.Context, page domain.Page, filter domain.UserFilter) (domain.PageResult[domain.User], error)
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	ByEmail(ctx context.Context, email string) (domain.User, error)
	Create(ctx context.Context, u domain.User) (domain.User, error)
	Update(ctx context.Context, id uuid.UUID, apply func(*domain.User)) (domain.User, error)
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

type Service interface {
	List(ctx context.Context, page domain.Page, filter domain.UserFilter) (domain.PageResult[domain.User], error)
	Get(ctx context.Context, id uuid.UUID) (domain.User, error)
	Create(ctx context.Context, input CreateInput) (domain.User, error)
	Update(ctx context.Context, actor domain.Identity, id uuid.UUID, patch Patch) (domain.User, error)
	ChangePassword(ctx context.Context, actor domain.Identity, id uuid.UUID, current, next string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CreateInput is an admin-side user creation. The password arrives in plaintext
// and is hashed before it reaches the entity, so no caller can hand this
// service an already-hashed value and have it stored unchanged.
type CreateInput struct {
	Email       string
	Password    string
	FirstName   string
	LastName    string
	AvatarURL   *string
	PhoneNumber *string
	Role        string
	Status      string
}

// Patch is a partial update. Role and Status are here but are only honoured for
// an admin -- see Service.Update.
type Patch struct {
	Email           *string
	FirstName       *string
	LastName        *string
	AvatarURL       *string
	PhoneNumber     *string
	Role            *string
	Status          *string
	IsEmailVerified *bool
}

func (p Patch) Empty() bool {
	return p.Email == nil && p.FirstName == nil && p.LastName == nil &&
		p.AvatarURL == nil && p.PhoneNumber == nil && p.Role == nil &&
		p.Status == nil && p.IsEmailVerified == nil
}

// Privileged reports whether this patch touches a field only an admin may set.
// Role, status and verified-ness are authorization state: a user who could set
// their own role could make themselves an admin.
func (p Patch) Privileged() bool {
	return p.Role != nil || p.Status != nil || p.IsEmailVerified != nil
}

func (p Patch) apply(target *domain.User) {
	if p.Email != nil {
		target.Email = *p.Email
	}
	if p.FirstName != nil {
		target.FirstName = *p.FirstName
	}
	if p.LastName != nil {
		target.LastName = *p.LastName
	}
	if p.AvatarURL != nil {
		target.AvatarURL = p.AvatarURL
	}
	if p.PhoneNumber != nil {
		target.PhoneNumber = p.PhoneNumber
	}
	if p.Role != nil {
		target.Role = *p.Role
	}
	if p.Status != nil {
		target.Status = *p.Status
	}
	if p.IsEmailVerified != nil {
		target.IsEmailVerified = *p.IsEmailVerified
	}
}
