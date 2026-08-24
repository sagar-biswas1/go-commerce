package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-commerce/domain"

	"github.com/google/uuid"
)

type fakeUsers struct {
	stored     *domain.User
	created    domain.User
	err        error
	loginTouch int
}

func (f *fakeUsers) ByID(context.Context, uuid.UUID) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	row := *f.stored
	return &row, nil
}

func (f *fakeUsers) ByEmail(context.Context, string) (*domain.User, error) {
	return f.ByID(context.Background(), uuid.Nil)
}

func (f *fakeUsers) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	f.created = *u
	stored := *u
	stored.ID = uuid.New()
	return &stored, f.err
}

func (f *fakeUsers) TouchLastLogin(context.Context, uuid.UUID) error {
	f.loginTouch++
	return nil
}

// fakeTokens records what the service asked of the store, which is where the
// rotation rules are visible.
type fakeTokens struct {
	familyID     uuid.UUID
	created      *domain.RefreshToken
	rotatedFrom  string
	rotatedNext  *domain.RefreshToken
	rotateErr    error
	familyErr    error
	revokedFam   uuid.UUID
	revokedToken string
}

func (f *fakeTokens) Create(_ context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	f.created = t
	stored := *t
	stored.ID = uuid.New()
	return &stored, nil
}

func (f *fakeTokens) ByToken(context.Context, string) (*domain.RefreshToken, error) {
	return nil, domain.ErrRefreshTokenNotFound
}

func (f *fakeTokens) FamilyOf(context.Context, string) (uuid.UUID, error) {
	if f.familyErr != nil {
		return uuid.Nil, f.familyErr
	}
	return f.familyID, nil
}

func (f *fakeTokens) Rotate(_ context.Context, old string, next *domain.RefreshToken) (*domain.RefreshToken, error) {
	f.rotatedFrom, f.rotatedNext = old, next
	if f.rotateErr != nil {
		return nil, f.rotateErr
	}
	stored := *next
	stored.ID = uuid.New()
	return &stored, nil
}

func (f *fakeTokens) RevokeByToken(_ context.Context, token string) error {
	f.revokedToken = token
	return nil
}

func (f *fakeTokens) RevokeByID(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakeTokens) RevokeFamily(_ context.Context, familyID uuid.UUID) (int64, error) {
	f.revokedFam = familyID
	return 3, nil
}

func (f *fakeTokens) RevokeAllForUser(context.Context, uuid.UUID) (int64, error) { return 1, nil }

func (f *fakeTokens) ActiveByUser(context.Context, uuid.UUID, domain.Page) (*domain.PageResult[*domain.RefreshToken], error) {
	return &domain.PageResult[*domain.RefreshToken]{}, nil
}

func (f *fakeTokens) DeleteExpired(context.Context, time.Duration) (int64, error) { return 0, nil }

// fakeIssuer records the family each refresh token was minted for, which is what
// makes the truthfulness of that claim testable.
type fakeIssuer struct {
	mintedFamily uuid.UUID
	parseErr     error
	parsedUserID uuid.UUID
}

func (f *fakeIssuer) IssueAccessToken(uuid.UUID, string) (string, time.Time, error) {
	return "access", time.Now().Add(15 * time.Minute), nil
}

func (f *fakeIssuer) IssueRefreshToken(_ uuid.UUID, familyID uuid.UUID) (string, time.Time, error) {
	f.mintedFamily = familyID
	return "refresh", time.Now().Add(24 * time.Hour), nil
}

func (f *fakeIssuer) ParseAccessToken(string) (domain.Identity, error) {
	return domain.Identity{}, nil
}

func (f *fakeIssuer) ParseRefreshToken(string) (uuid.UUID, error) {
	return f.parsedUserID, f.parseErr
}

type fakeHasher struct{ compareErr error }

func (fakeHasher) Hash(plaintext string) (string, error) { return "hashed:" + plaintext, nil }

func (f fakeHasher) Compare(hashed, plaintext string) error {
	if f.compareErr != nil {
		return f.compareErr
	}
	if hashed != "hashed:"+plaintext {
		return errors.New("mismatch")
	}
	return nil
}

func newFixture() (*service, *fakeUsers, *fakeTokens, *fakeIssuer) {
	id := uuid.New()
	users := &fakeUsers{stored: &domain.User{
		ID:        id,
		Email:     "sagar@example.com",
		Password:  "hashed:GoodPass1",
		FirstName: "Sagar",
		LastName:  "Biswas",
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	}}
	tokens := &fakeTokens{familyID: uuid.New()}
	issuer := &fakeIssuer{parsedUserID: id}

	return &service{users: users, tokens: tokens, issuer: issuer, hasher: fakeHasher{}}, users, tokens, issuer
}

// Registration always makes an ordinary user. The input type has no role field,
// and this is the test that the service does not invent one either.
func TestRegisterAlwaysCreatesAnOrdinaryUser(t *testing.T) {
	svc, users, _, _ := newFixture()

	_, err := svc.Register(context.Background(), &domain.RegisterInput{
		FirstName: "New", LastName: "Person",
		Email: "new@example.com", Password: "GoodPass1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if users.created.Role != domain.RoleUser {
		t.Errorf("role = %q, want %q", users.created.Role, domain.RoleUser)
	}
	if users.created.Password == "GoodPass1" {
		t.Fatal("the plaintext password reached storage")
	}
}

func TestLoginRejectsAWrongPassword(t *testing.T) {
	svc, _, _, _ := newFixture()
	svc.hasher = fakeHasher{compareErr: errors.New("mismatch")}

	_, _, err := svc.Login(context.Background(), "sagar@example.com", "wrong", nil)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

// An unknown address and a wrong password must be one error, or the difference
// tells anyone which addresses have accounts.
func TestLoginDoesNotRevealWhetherAnAddressExists(t *testing.T) {
	svc, users, _, _ := newFixture()
	users.err = domain.ErrUserNotFound

	_, _, err := svc.Login(context.Background(), "nobody@example.com", "GoodPass1", nil)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want the same error a wrong password gives", err)
	}
}

// The password was right, so the account itself is the honest answer -- the
// caller has already proved they own it.
func TestLoginRefusesASuspendedAccount(t *testing.T) {
	svc, users, _, _ := newFixture()
	users.stored.Status = domain.StatusSuspended

	_, _, err := svc.Login(context.Background(), "sagar@example.com", "GoodPass1", nil)
	if !errors.Is(err, domain.ErrUserNotActive) {
		t.Fatalf("err = %v, want ErrUserNotActive", err)
	}
}

func TestLoginStartsAFreshFamily(t *testing.T) {
	svc, _, tokens, issuer := newFixture()

	_, pair, err := svc.Login(context.Background(), "sagar@example.com", "GoodPass1",
		&domain.SessionContext{UserAgent: "curl", IPAddress: "10.0.0.1"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("pair = %+v, want both tokens", pair)
	}
	if tokens.created == nil {
		t.Fatal("no refresh token was stored")
	}
	// Only the digest is kept: a copy of the database must not be a set of
	// working sessions.
	if tokens.created.TokenHash != domain.HashToken(pair.RefreshToken) {
		t.Error("the stored row does not hold the digest of the issued token")
	}
	if tokens.created.TokenHash == pair.RefreshToken {
		t.Fatal("the refresh token was stored in the clear")
	}
	if issuer.mintedFamily == uuid.Nil || issuer.mintedFamily == tokens.familyID {
		t.Errorf("family = %v, want a new one for a fresh login", issuer.mintedFamily)
	}
}

// The successor's own claim has to name the chain it actually extends. Minting
// it against a fresh random family would make the claim a lie the moment the
// store inherited the real one.
func TestRefreshMintsIntoTheExistingFamily(t *testing.T) {
	svc, _, tokens, issuer := newFixture()

	_, _, err := svc.Refresh(context.Background(), "old-token", nil)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if issuer.mintedFamily != tokens.familyID {
		t.Errorf("minted family = %v, want the presented token's family %v",
			issuer.mintedFamily, tokens.familyID)
	}
	if tokens.rotatedFrom != "old-token" {
		t.Errorf("rotated from %q, want the presented token", tokens.rotatedFrom)
	}
}

// A replayed token means it was copied -- by whoever holds it now or whoever
// held it before. There is no way to tell which, so the family goes.
func TestRefreshRevokesTheFamilyOnReuse(t *testing.T) {
	svc, _, tokens, _ := newFixture()
	tokens.rotateErr = domain.ErrRefreshTokenReused

	_, _, err := svc.Refresh(context.Background(), "replayed", nil)
	if !errors.Is(err, domain.ErrRefreshTokenReused) {
		t.Fatalf("err = %v, want ErrRefreshTokenReused", err)
	}
	if tokens.revokedFam != tokens.familyID {
		t.Errorf("revoked family = %v, want %v", tokens.revokedFam, tokens.familyID)
	}
}

// An ordinary expired session is not a reuse, so it must not take a family with
// it: that would sign out a legitimate user for waiting too long.
func TestRefreshLeavesTheFamilyAloneWhenTheTokenMerelyExpired(t *testing.T) {
	svc, _, tokens, _ := newFixture()
	tokens.rotateErr = domain.ErrRefreshTokenExpired

	_, _, err := svc.Refresh(context.Background(), "stale", nil)
	if !errors.Is(err, domain.ErrRefreshTokenExpired) {
		t.Fatalf("err = %v, want ErrRefreshTokenExpired", err)
	}
	if tokens.revokedFam != uuid.Nil {
		t.Error("an expired token revoked the whole family")
	}
}

// A forged token must never reach the database, where it could be used to probe
// which tokens exist.
func TestRefreshChecksTheSignatureBeforeTheStore(t *testing.T) {
	svc, _, tokens, issuer := newFixture()
	issuer.parseErr = domain.ErrUnauthorized

	if _, _, err := svc.Refresh(context.Background(), "forged", nil); err == nil {
		t.Fatal("a forged token was accepted")
	}
	if tokens.rotatedFrom != "" {
		t.Error("a forged token reached the store")
	}
}

// The account was suspended since the token was minted: retire the session
// rather than renewing it.
func TestRefreshRetiresTheSessionOfASuspendedAccount(t *testing.T) {
	svc, users, tokens, _ := newFixture()
	users.stored.Status = domain.StatusSuspended

	_, _, err := svc.Refresh(context.Background(), "token", nil)
	if !errors.Is(err, domain.ErrUserNotActive) {
		t.Fatalf("err = %v, want ErrUserNotActive", err)
	}
	if tokens.revokedToken != "token" {
		t.Error("the session of a suspended account was left live")
	}
}

// Logging out of a session that is already gone is the outcome the caller
// wanted, and distinguishing the cases would say which tokens exist.
func TestLogoutIsSilentAboutUnknownTokens(t *testing.T) {
	svc, _, _, _ := newFixture()

	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("empty token: %v", err)
	}
}

func TestLogoutRefusesToRevokeOnAnUnsignedString(t *testing.T) {
	svc, _, tokens, issuer := newFixture()
	issuer.parseErr = domain.ErrUnauthorized

	if err := svc.Logout(context.Background(), "not-a-token"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if tokens.revokedToken != "" {
		t.Error("an unsigned string reached a write; anyone could revoke by guessing")
	}
}
