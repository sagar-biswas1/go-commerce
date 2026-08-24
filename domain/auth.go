package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefreshToken is one issued refresh credential.
//
// It holds TokenHash, not the token. Storing a digest means a copy of the
// database is not a set of working sessions -- the same reason the password
// column holds a bcrypt digest and not a password.
//
// FamilyID groups every token descended from one login. Rotation spends the
// presented token and issues its successor into the same family, which is what
// makes a stolen token detectable however many rotations later it is replayed.
type RefreshToken struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"userId" db:"user_id"`
	FamilyID   uuid.UUID  `json:"familyId" db:"family_id"`
	TokenHash  string     `json:"-" db:"token_hash"`
	UserAgent  *string    `json:"userAgent,omitempty" db:"user_agent"`
	IPAddress  *string    `json:"ipAddress,omitempty" db:"ip_address"`
	ExpiresAt  time.Time  `json:"expiresAt" db:"expires_at"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty" db:"revoked_at"`
	ReplacedBy *uuid.UUID `json:"replacedBy,omitempty" db:"replaced_by"`
	CreatedAt  time.Time  `json:"createdAt" db:"created_at"`
}

var (
	ErrRefreshTokenNotFound = wrap("refresh token not found", ErrNotFound)
	ErrRefreshTokenExpired  = wrap("refresh token has expired", ErrUnauthorized)

	// ErrRefreshTokenReused means a token that was already spent came back.
	//
	// A well-behaved client presents each refresh token exactly once, so a
	// second presentation says the token was copied -- by whoever holds it now,
	// or by whoever held it before. There is no way to tell which, so the whole
	// family is revoked and both parties are made to sign in again.
	ErrRefreshTokenReused = wrap("refresh token has already been used", ErrUnauthorized)
)

// HashToken reduces a refresh token to what is stored.
//
// A plain digest rather than bcrypt, on purpose: the token is already a signed
// value with far more entropy than a password, so there is nothing to
// brute-force, and lookup has to be an indexed equality match rather than a
// scan comparing every row.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsActive reports whether this token can still be exchanged.
func (t RefreshToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// TokenPair is what a successful login or refresh hands back. The refresh token
// is returned in plaintext exactly once -- at the moment it is minted -- because
// only its digest is kept.
type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// Identity is the authenticated caller, as carried on a request. It is the
// subset of a user that an access token asserts, which is all a middleware can
// know without going back to the database.
type Identity struct {
	UserID uuid.UUID
	Role   string
}

func (i Identity) IsAdmin() bool { return i.Role == RoleAdmin }

// CanActOn decides whether this caller may read or change the resource owned by
// owner. Admins may act on anyone; everybody else only on themselves.
//
// It lives here, next to the roles it reads, because every handler that touches
// somebody else's data needs the same answer, and each writing its own is how
// one of them ends up subtly more permissive than the rest.
//
// A zero Identity -- what an unauthenticated context yields -- can act on
// nothing, including the nil UUID it would otherwise compare equal to.
func (i Identity) CanActOn(owner uuid.UUID) bool {
	if i.UserID == uuid.Nil {
		return false
	}
	return i.IsAdmin() || i.UserID == owner
}

// wrap builds a domain error that reads as its own message but still matches a
// category with errors.Is.
func wrap(message string, category error) error {
	return fmt.Errorf("%s: %w", message, category)
}

// RegisterInput is a public registration.
//
// There is no role field, and that is the point: a new account is always an
// ordinary user. A type with nowhere to put a role is a stronger guarantee than
// a service that remembers to ignore one.
type RegisterInput struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
}

// SessionContext is what a client incidentally tells us about where a session
// lives. It is recorded so a "signed in on these devices" screen has something
// to show, and is never trusted for authorization -- both values are
// attacker-controlled.
type SessionContext struct {
	UserAgent string
	IPAddress string
}
