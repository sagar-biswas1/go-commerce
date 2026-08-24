package security

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-commerce/config"
	"go-commerce/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTIssuer mints and validates the two token kinds. It satisfies the
// TokenIssuer port the auth service declares.
type JWTIssuer struct {
	cfg *config.JWTConfig
}

var (
	jwtOnce     sync.Once
	jwtInstance *JWTIssuer
)

// GetJWTIssuer returns the process-wide issuer, built on the first call from the
// config it is given. Like the other singletons here it takes its dependency
// rather than reaching for it, so the composition root stays the only place
// that decides what anything is configured with.
func GetJWTIssuer(cfg *config.JWTConfig) *JWTIssuer {
	jwtOnce.Do(func() {
		jwtInstance = NewJWTIssuer(cfg)
	})
	return jwtInstance
}

func NewJWTIssuer(cfg *config.JWTConfig) *JWTIssuer {
	return &JWTIssuer{cfg: cfg}
}

// signingMethod is the one algorithm this service signs and accepts.
//
// Naming it matters. A validator that trusts the token's own `alg` header can be
// handed a token that says `alg: none`, or an RS256 verifier can be handed an
// HS256 token signed with its public key -- the two classic JWT forgeries. Both
// are refused by only ever accepting this one method, whatever the header claims.
var signingMethod = jwt.SigningMethodHS256

// tokenKind separates the two token types inside the payload.
//
// The secrets already differ, so a refresh token cannot verify as an access
// token. This is the second lock on the same door: a claim naming its own
// purpose means a token is refused by the wrong endpoint even if the two keys
// were ever misconfigured to match.
const (
	kindAccess  = "access"
	kindRefresh = "refresh"
)

// AccessClaims is what an access token asserts. It is deliberately small: every
// field here is something a middleware will trust without asking the database,
// so anything that can change -- a name, an email -- does not belong in it.
type AccessClaims struct {
	Kind string `json:"knd"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims carries the family, so a rotation knows which chain it extends
// without a lookup, and a jti, so two tokens minted in the same second for the
// same user are still distinct values.
type RefreshClaims struct {
	Kind     string `json:"knd"`
	FamilyID string `json:"fid"`
	jwt.RegisteredClaims
}

func (i *JWTIssuer) accessSecret() []byte  { return []byte(strings.TrimSpace(i.cfg.AccessSecret)) }
func (i *JWTIssuer) refreshSecret() []byte { return []byte(strings.TrimSpace(i.cfg.RefreshSecret)) }

// IssueAccessToken mints a short-lived token asserting who the caller is.
func (i *JWTIssuer) IssueAccessToken(userID uuid.UUID, role string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(i.cfg.AccessTTL)

	claims := AccessClaims{
		Kind: kindAccess,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.cfg.Issuer,
			Audience:  jwt.ClaimStrings{i.cfg.Audience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(i.accessSecret())
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing access token: %w", err)
	}
	return signed, expiresAt, nil
}

// IssueRefreshToken mints a long-lived token. A nil family id starts a new
// family, which is what a fresh login does.
func (i *JWTIssuer) IssueRefreshToken(userID, familyID uuid.UUID) (string, time.Time, error) {
	if familyID == uuid.Nil {
		familyID = uuid.New()
	}

	now := time.Now().UTC()
	expiresAt := now.Add(i.cfg.RefreshTTL)

	claims := RefreshClaims{
		Kind:     kindRefresh,
		FamilyID: familyID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  userID.String(),
			Issuer:   i.cfg.Issuer,
			Audience: jwt.ClaimStrings{i.cfg.Audience},
			// The jti is what makes two tokens minted in the same second for
			// the same user hash differently, so the token_hash unique index
			// cannot collide on a fast rotation.
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(i.refreshSecret())
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing refresh token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken validates a token and returns who it says the caller is.
//
// Every check the library offers is switched on, because each one closes a way
// a token from somewhere else could be accepted here: the method, so the header
// cannot choose the algorithm; the issuer and audience, so a token minted by
// another service sharing this secret is refused; a required expiry, so a token
// that simply omits `exp` does not read as one that never expires.
func (i *JWTIssuer) ParseAccessToken(rawToken string) (domain.Identity, error) {
	claims := &AccessClaims{}

	parsed, err := jwt.ParseWithClaims(rawToken, claims,
		func(*jwt.Token) (any, error) { return i.accessSecret(), nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(i.cfg.Issuer),
		jwt.WithAudience(i.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(i.cfg.Leeway),
	)
	if err != nil || !parsed.Valid {
		return domain.Identity{}, domain.ErrUnauthorized
	}

	if claims.Kind != kindAccess {
		return domain.Identity{}, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return domain.Identity{}, domain.ErrUnauthorized
	}

	if _, ok := domain.ValidateRole(claims.Role); !ok {
		// A role the domain does not recognise cannot be authorized against, and
		// a token carrying one was not minted by a build that agrees with this
		// one about what roles mean.
		return domain.Identity{}, domain.ErrUnauthorized
	}

	return domain.Identity{UserID: userID, Role: claims.Role}, nil
}

// ParseRefreshToken validates the signature and expiry and returns whose token
// it is. Whether the token is still spendable is a question for the store, not
// for the signature.
func (i *JWTIssuer) ParseRefreshToken(rawToken string) (uuid.UUID, error) {
	claims := &RefreshClaims{}

	parsed, err := jwt.ParseWithClaims(rawToken, claims,
		func(*jwt.Token) (any, error) { return i.refreshSecret(), nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(i.cfg.Issuer),
		jwt.WithAudience(i.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(i.cfg.Leeway),
	)
	if err != nil {
		if isExpired(err) {
			return uuid.Nil, domain.ErrRefreshTokenExpired
		}
		return uuid.Nil, domain.ErrUnauthorized
	}
	if !parsed.Valid || claims.Kind != kindRefresh {
		return uuid.Nil, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return userID, nil
}

// isExpired distinguishes an expired token from an invalid one, so a client can
// be told to refresh rather than to sign in again. jwt/v5 joins its validation
// failures, so errors.Is finds this one among several.
func isExpired(err error) bool {
	return errors.Is(err, jwt.ErrTokenExpired)
}
