package auth

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"

	"go-commerce/domain"

	"github.com/google/uuid"
)

type service struct {
	users  UserReader
	tokens TokenStore
	issuer TokenIssuer
	hasher PasswordHasher
}

var (
	once     sync.Once
	instance Service
)

// GetService returns the process-wide auth service.
func GetService(users UserReader, tokens TokenStore, issuer TokenIssuer, hasher PasswordHasher) Service {
	once.Do(func() {
		instance = NewService(users, tokens, issuer, hasher)
	})
	return instance
}

func NewService(users UserReader, tokens TokenStore, issuer TokenIssuer, hasher PasswordHasher) Service {
	return &service{users: users, tokens: tokens, issuer: issuer, hasher: hasher}
}

// Register creates an account.
//
// Role is not taken from the request. The old handler accepted whatever role the
// body named, which let anyone register as an admin; a new account is always an
// ordinary user, and promoting one is an admin's job.
func (s *service) Register(ctx context.Context, input RegisterInput) (domain.User, error) {
	newUser := domain.User{
		Email:     input.Email,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	}
	newUser.Normalize()

	if err := newUser.Validate(input.Password); err != nil {
		return domain.User{}, err
	}

	hashed, err := s.hasher.Hash(input.Password)
	if err != nil {
		return domain.User{}, err
	}
	newUser.Password = hashed

	return s.users.Create(ctx, newUser)
}

// Login exchanges credentials for a token pair.
func (s *service) Login(ctx context.Context, email, password string, session SessionContext) (domain.User, domain.TokenPair, error) {
	found, err := s.users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Compare against nothing anyway, so a request for an address with
			// no account takes about as long as one for an address with a wrong
			// password. Returning early here is what turns a timing difference
			// into a way to enumerate registered addresses.
			_ = s.hasher.Compare(dummyHash, password)
			return domain.User{}, domain.TokenPair{}, domain.ErrInvalidCredentials
		}
		return domain.User{}, domain.TokenPair{}, err
	}

	if err := s.hasher.Compare(found.Password, password); err != nil {
		return domain.User{}, domain.TokenPair{}, domain.ErrInvalidCredentials
	}

	// The password was right, so say plainly that the account itself is the
	// problem -- the caller has already proved they own it.
	if !found.CanAuthenticate() {
		return domain.User{}, domain.TokenPair{}, domain.ErrUserNotActive
	}

	// A fresh login starts a new family: it is a new session, unrelated to
	// whatever else this user has open.
	pair, err := s.issuePair(ctx, found, uuid.New(), "", session)
	if err != nil {
		return domain.User{}, domain.TokenPair{}, err
	}

	if err := s.users.TouchLastLogin(ctx, found.ID); err != nil {
		// The login succeeded; a missing timestamp is not worth failing it.
		log.Printf("[auth] could not record login for %s: %v", found.ID, err)
	}

	return found, pair, nil
}

// Refresh rotates a refresh token: the presented token is spent and a new pair
// is issued in the same family.
//
// The order of checks here is the security-relevant part.
//
//  1. Signature and expiry first, so a forged or stale token never reaches the
//     database and cannot be used to probe it.
//  2. Then the store, which spends the token conditionally. That UPDATE is what
//     detects reuse -- not a preceding SELECT, which two concurrent requests
//     could both pass.
//  3. A reused token revokes the whole family. Either the token was stolen and
//     the thief got there first, or the legitimate client is replaying one the
//     thief already spent. There is no way to tell which, so both are signed
//     out and the real owner logs in again.
func (s *service) Refresh(ctx context.Context, refreshToken string, session SessionContext) (domain.User, domain.TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return domain.User{}, domain.TokenPair{}, domain.ErrRefreshTokenNotFound
	}

	userID, err := s.issuer.ParseRefreshToken(refreshToken)
	if err != nil {
		return domain.User{}, domain.TokenPair{}, err
	}

	found, err := s.users.ByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, domain.TokenPair{}, domain.ErrUnauthorized
		}
		return domain.User{}, domain.TokenPair{}, err
	}
	if !found.CanAuthenticate() {
		// The account was suspended or deleted since this token was minted.
		// Retire the session rather than renewing it.
		s.revokeQuietly(ctx, refreshToken)
		return domain.User{}, domain.TokenPair{}, domain.ErrUserNotActive
	}

	// Read the family before rotating, so the successor's own claim names the
	// chain it actually extends rather than inventing a new one. This is a read
	// before a write, but it is not a check-then-act: the conditional UPDATE
	// inside Rotate is still the only thing that decides whether the token was
	// spendable, so two concurrent requests cannot both succeed.
	familyID, err := s.tokens.FamilyOf(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Correctly signed, but no such row: either it was swept long after
			// expiry, or it was minted by a build using a different store.
			return domain.User{}, domain.TokenPair{}, domain.ErrRefreshTokenNotFound
		}
		return domain.User{}, domain.TokenPair{}, err
	}

	pair, err := s.issuePair(ctx, found, familyID, refreshToken, session)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenReused) {
			s.revokeFamily(ctx, familyID, found.ID)
		}
		return domain.User{}, domain.TokenPair{}, err
	}

	return found, pair, nil
}

// issuePair mints an access token and a refresh token together.
//
// When previousToken is empty this starts a family; otherwise it rotates,
// spending previousToken in the same transaction that stores the successor. The
// refresh JWT is minted before the row is written because the row stores its
// digest -- so a failure to store means the token was never handed out, rather
// than a live token nobody is tracking.
func (s *service) issuePair(
	ctx context.Context,
	forUser domain.User,
	familyID uuid.UUID,
	previousToken string,
	session SessionContext,
) (domain.TokenPair, error) {
	accessToken, accessExpiry, err := s.issuer.IssueAccessToken(forUser.ID, forUser.Role)
	if err != nil {
		return domain.TokenPair{}, err
	}

	refreshToken, refreshExpiry, err := s.issuer.IssueRefreshToken(forUser.ID, familyID)
	if err != nil {
		return domain.TokenPair{}, err
	}

	record := domain.RefreshToken{
		UserID:    forUser.ID,
		FamilyID:  familyID,
		TokenHash: domain.HashToken(refreshToken),
		UserAgent: optional(session.UserAgent),
		IPAddress: optional(session.IPAddress),
		ExpiresAt: refreshExpiry,
	}

	if previousToken == "" {
		if _, err := s.tokens.Create(ctx, record); err != nil {
			return domain.TokenPair{}, err
		}
	} else if _, err := s.tokens.Rotate(ctx, previousToken, record); err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiry,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiry,
	}, nil
}

// Logout ends the one session the presented token belongs to.
//
// It reports no error for a token that is unknown or already spent: logging out
// of a session that is already gone is the outcome the caller wanted, and
// telling them apart would say which tokens exist.
func (s *service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}

	// Validate the signature first: without it, anyone could revoke by
	// guessing, and an unsigned string should never reach a write.
	if _, err := s.issuer.ParseRefreshToken(refreshToken); err != nil {
		return nil
	}

	if err := s.tokens.RevokeByToken(ctx, refreshToken); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	return nil
}

func (s *service) LogoutEverywhere(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.tokens.RevokeAllForUser(ctx, userID)
}

func (s *service) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.users.ByID(ctx, userID)
}

func (s *service) Sessions(ctx context.Context, userID uuid.UUID, page domain.Page) (domain.PageResult[domain.RefreshToken], error) {
	return s.tokens.ActiveByUser(ctx, userID, page)
}

// RevokeSession ends one named session. The user id is part of the store's
// condition, so a client can only end its own sessions whatever id it guesses.
func (s *service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.tokens.RevokeByID(ctx, sessionID, userID)
}

func (s *service) revokeQuietly(ctx context.Context, refreshToken string) {
	if err := s.tokens.RevokeByToken(ctx, refreshToken); err != nil && !errors.Is(err, domain.ErrNotFound) {
		log.Printf("[auth] could not revoke refresh token: %v", err)
	}
}

// revokeFamily ends every session descended from the same login. This is the
// response to a replayed token, so it is logged loudly: it is the signal that a
// refresh token leaked, and nothing else in the system will say so.
func (s *service) revokeFamily(ctx context.Context, familyID, userID uuid.UUID) {
	revoked, err := s.tokens.RevokeFamily(ctx, familyID)
	if err != nil {
		log.Printf("[auth] SECURITY: refresh token reuse for user %s, family %s could not be revoked: %v",
			userID, familyID, err)
		return
	}

	log.Printf("[auth] SECURITY: refresh token reuse detected for user %s; revoked %d token(s) in family %s",
		userID, revoked, familyID)
}

// optional turns an empty string into a nil pointer, so a column stays NULL
// rather than holding "".
func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// dummyHash is a real bcrypt digest of a value nobody knows, compared against
// when no user was found so the work done is the same either way. Its cost has
// to match what the hasher produces, or the timing difference comes back.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
