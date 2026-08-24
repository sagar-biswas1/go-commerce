package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// fakeRepo is why Repository is declared in this package: the rules below are
// testable with no database, no migration and no container.
type fakeRepo struct {
	stored  domain.User
	created domain.User
	applied domain.User
	err     error

	updateSeen bool
}

func (f *fakeRepo) All(context.Context, domain.Page, *domain.UserFilter) (*domain.PageResult[*domain.User], error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.PageResult[*domain.User]{Items: []*domain.User{&f.stored}, Total: 1}, nil
}

func (f *fakeRepo) ByID(context.Context, uuid.UUID) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	// A copy, as a real repository returns: a caller must not be able to edit
	// the stored row by writing through what it was handed.
	row := f.stored
	return &row, nil
}

func (f *fakeRepo) ByEmail(context.Context, string) (*domain.User, error) {
	return f.ByID(context.Background(), uuid.Nil)
}

func (f *fakeRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	f.created = *u
	stored := *u
	stored.ID = uuid.New()
	return &stored, f.err
}

func (f *fakeRepo) Update(_ context.Context, _ uuid.UUID, apply func(*domain.User)) (*domain.User, error) {
	f.updateSeen = true
	row := f.stored
	apply(&row)
	f.applied = row
	if f.err != nil {
		return nil, f.err
	}
	return &row, nil
}

func (f *fakeRepo) Delete(context.Context, uuid.UUID) error         { return f.err }
func (f *fakeRepo) TouchLastLogin(context.Context, uuid.UUID) error { return f.err }

// fakeHasher stands in for bcrypt. Real bcrypt at production cost makes a suite
// that creates users slow enough that people stop running it.
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

type fakeRevoker struct{ calls int }

func (f *fakeRevoker) RevokeAllForUser(context.Context, uuid.UUID) (int64, error) {
	f.calls++
	return 2, nil
}

func newFixture() (*service, *fakeRepo, *fakeRevoker) {
	repo := &fakeRepo{stored: domain.User{
		ID:        uuid.New(),
		Email:     "sagar@example.com",
		Password:  "hashed:CurrentPass1",
		FirstName: "Sagar",
		LastName:  "Biswas",
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	}}
	sessions := &fakeRevoker{}
	return &service{repo: repo, hasher: fakeHasher{}, sessions: sessions}, repo, sessions
}

func admin() domain.Identity { return domain.Identity{UserID: uuid.New(), Role: domain.RoleAdmin} }

func self(id uuid.UUID) domain.Identity {
	return domain.Identity{UserID: id, Role: domain.RoleUser}
}

// The entity holds a digest and the input holds plaintext, which is the whole
// reason they are two types. This is the test that the boundary is respected.
func TestCreateHashesThePasswordBeforeStoring(t *testing.T) {
	svc, repo, _ := newFixture()

	_, err := svc.Create(context.Background(), &domain.UserCreateInput{
		Email:     "New@Example.com ",
		Password:  "GoodPass1",
		FirstName: "New",
		LastName:  "Person",
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if repo.created.Password == "GoodPass1" {
		t.Fatal("the plaintext password reached the repository")
	}
	if !strings.HasPrefix(repo.created.Password, "hashed:") {
		t.Errorf("stored password = %q, want a digest", repo.created.Password)
	}
	if repo.created.Email != "new@example.com" {
		t.Errorf("stored email = %q, want it lowered and trimmed", repo.created.Email)
	}
}

func TestCreateRejectsAWeakPassword(t *testing.T) {
	svc, repo, _ := newFixture()

	_, err := svc.Create(context.Background(), &domain.UserCreateInput{
		Email:     "new@example.com",
		Password:  "short",
		FirstName: "New",
		LastName:  "Person",
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want an ErrInvalid", err)
	}
	if repo.created.Email != "" {
		t.Error("an invalid user reached the repository")
	}
}

// Role, status and verified-ness are authorization state. A user who could set
// their own role could make themselves an admin, so this is the rule that has to
// hold even though the transport accepts the fields.
func TestUpdateRefusesPrivilegedFieldsFromANonAdmin(t *testing.T) {
	svc, repo, _ := newFixture()
	adminRole := domain.RoleAdmin

	_, err := svc.Update(context.Background(), self(repo.stored.ID), repo.stored.ID,
		&domain.UserPatch{Role: &adminRole})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if repo.updateSeen {
		t.Error("a privilege escalation reached the write")
	}
}

func TestUpdateLetsAnAdminSetARole(t *testing.T) {
	svc, repo, sessions := newFixture()
	staff := domain.RoleStaff

	updated, err := svc.Update(context.Background(), admin(), repo.stored.ID,
		&domain.UserPatch{Role: &staff})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Role != domain.RoleStaff {
		t.Errorf("role = %q, want %q", updated.Role, domain.RoleStaff)
	}
	// The tokens already issued assert the old role, so they have to go or the
	// change does not take effect until the last one lapses.
	if sessions.calls != 1 {
		t.Errorf("sessions revoked %d times, want 1 after a role change", sessions.calls)
	}
}

func TestUpdateRefusesActingOnSomebodyElse(t *testing.T) {
	svc, repo, _ := newFixture()
	name := "Someone"

	_, err := svc.Update(context.Background(), self(uuid.New()), repo.stored.ID,
		&domain.UserPatch{FirstName: &name})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

// An unauthenticated context yields a zero Identity, whose nil UUID would
// otherwise compare equal to a nil owner id.
func TestUpdateRefusesAZeroIdentity(t *testing.T) {
	svc, _, _ := newFixture()
	name := "Someone"

	_, err := svc.Update(context.Background(), domain.Identity{}, uuid.Nil,
		&domain.UserPatch{FirstName: &name})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestUpdateRejectsAPatchThatAsksForNothing(t *testing.T) {
	for name, patch := range map[string]*domain.UserPatch{
		"empty": {},
		"nil":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo, _ := newFixture()

			_, err := svc.Update(context.Background(), admin(), repo.stored.ID, patch)
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("err = %v, want an ErrInvalid", err)
			}
			if repo.updateSeen {
				t.Error("a patch asking for nothing still took a write lock")
			}
		})
	}
}

// The dry-run merge must not reach the row the locked write is based on.
func TestUpdatePreviewDoesNotLeakIntoTheWrite(t *testing.T) {
	svc, repo, _ := newFixture()
	name := "Renamed"

	if _, err := svc.Update(context.Background(), admin(), repo.stored.ID,
		&domain.UserPatch{FirstName: &name}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.stored.FirstName != "Sagar" {
		t.Errorf("stored row = %q, want the dry run to have left it untouched", repo.stored.FirstName)
	}
}

// A user must prove they know the current password: a stolen access token alone
// should not be enough to lock the owner out of their own account.
func TestChangePasswordRequiresTheCurrentOneFromAUser(t *testing.T) {
	svc, repo, sessions := newFixture()

	err := svc.ChangePassword(context.Background(), self(repo.stored.ID), repo.stored.ID,
		"WrongPass1", "NewGoodPass1")
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want a validation error naming the current password", err)
	}
	if sessions.calls != 0 {
		t.Error("a failed password change still ended the sessions")
	}
}

// An admin is exempt, so an account can be recovered without knowing it.
func TestChangePasswordLetsAnAdminSkipTheCurrentOne(t *testing.T) {
	svc, repo, sessions := newFixture()

	if err := svc.ChangePassword(context.Background(), admin(), repo.stored.ID,
		"", "NewGoodPass1"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if repo.applied.Password != "hashed:NewGoodPass1" {
		t.Errorf("stored password = %q, want the new digest", repo.applied.Password)
	}
	// Every existing session was authorized by the old password.
	if sessions.calls != 1 {
		t.Errorf("sessions revoked %d times, want 1 after a password change", sessions.calls)
	}
}

func TestChangePasswordRejectsAWeakNewPassword(t *testing.T) {
	svc, repo, _ := newFixture()

	err := svc.ChangePassword(context.Background(), admin(), repo.stored.ID, "", "short")
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want an ErrInvalid", err)
	}
}

// The row is soft-deleted, so nothing cascades: without this the deleted user
// keeps refreshing.
func TestDeleteEndsTheSessions(t *testing.T) {
	svc, repo, sessions := newFixture()

	if err := svc.Delete(context.Background(), repo.stored.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if sessions.calls != 1 {
		t.Errorf("sessions revoked %d times, want 1 after a delete", sessions.calls)
	}
}

func TestListRejectsAnUnknownRoleFilter(t *testing.T) {
	svc, _, _ := newFixture()

	_, err := svc.List(context.Background(), domain.NewPage(1, 20), &domain.UserFilter{Role: "wizard"})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want a filter that can never match to be refused", err)
	}
}
