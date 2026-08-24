package user

import (
	"context"
	"errors"
	"log"
	"sync"

	"go-commerce/domain"

	"github.com/google/uuid"
)

type service struct {
	repo     Repository
	hasher   PasswordHasher
	sessions SessionRevoker
}

var (
	once     sync.Once
	instance Service
)

// GetService returns the process-wide user service.
func GetService(repo Repository, hasher PasswordHasher, sessions SessionRevoker) Service {
	once.Do(func() {
		instance = NewService(repo, hasher, sessions)
	})
	return instance
}

func NewService(repo Repository, hasher PasswordHasher, sessions SessionRevoker) Service {
	return &service{repo: repo, hasher: hasher, sessions: sessions}
}

func (s *service) List(ctx context.Context, page domain.Page, filter domain.UserFilter) (domain.PageResult[domain.User], error) {
	if err := filter.Validate(); err != nil {
		return domain.PageResult[domain.User]{}, err
	}
	return s.repo.All(ctx, page, filter)
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return s.repo.ByID(ctx, id)
}

func (s *service) Create(ctx context.Context, input CreateInput) (domain.User, error) {
	newUser := domain.User{
		Email:       input.Email,
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		AvatarURL:   input.AvatarURL,
		PhoneNumber: input.PhoneNumber,
		Role:        input.Role,
		Status:      input.Status,
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

	// No pre-flight "does this email exist" query: two concurrent requests can
	// both pass such a check. The unique index decides, and the repository
	// turns its violation into ErrEmailTaken.
	return s.repo.Create(ctx, newUser)
}

// Update applies a partial change on behalf of actor.
//
// Authorization is decided here rather than in the handler because the rule
// depends on what is being changed, not just on who is asking: a user may edit
// their own profile, but role, status and email verification are authorization
// state and belong to an admin.
func (s *service) Update(ctx context.Context, actor domain.Identity, id uuid.UUID, patch Patch) (domain.User, error) {
	if patch.Empty() {
		v := domain.NewValidationError()
		v.Add("body", "no updatable fields were provided")
		return domain.User{}, v.OrNil()
	}
	if !actor.CanActOn(id) {
		return domain.User{}, domain.ErrForbidden
	}
	if patch.Privileged() && !actor.IsAdmin() {
		return domain.User{}, domain.ErrForbidden
	}

	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	preview := current
	patch.apply(&preview)
	preview.Normalize()
	if err := preview.ValidateProfile(); err != nil {
		return domain.User{}, err
	}

	updated, err := s.repo.Update(ctx, id, func(u *domain.User) {
		patch.apply(u)
		u.Normalize()
	})
	if err != nil {
		return domain.User{}, err
	}

	// A user who has just been suspended, deleted, or handed a different role
	// still holds refresh tokens minted under the old state. Ending those
	// sessions is what makes the change take effect now rather than whenever
	// the tokens happen to lapse.
	if roleOrStatusChanged(current, updated) {
		s.revokeSessions(ctx, id, "role or status changed")
	}

	return updated, nil
}

func roleOrStatusChanged(before, after domain.User) bool {
	return before.Role != after.Role || before.Status != after.Status
}

// ChangePassword replaces a password, requiring the current one from anybody
// changing their own.
//
// An admin is exempt, so an account can be recovered without knowing the
// password; a user is not, so a stolen access token alone cannot lock the owner
// out of their own account.
func (s *service) ChangePassword(ctx context.Context, actor domain.Identity, id uuid.UUID, current, next string) error {
	if !actor.CanActOn(id) {
		return domain.ErrForbidden
	}

	target, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}

	if !actor.IsAdmin() {
		if err := s.hasher.Compare(target.Password, current); err != nil {
			v := domain.NewValidationError()
			v.Add("currentPassword", "current password is incorrect")
			return v.OrNil()
		}
	}

	if msg, ok := domain.ValidatePassword(next); !ok {
		v := domain.NewValidationError()
		v.Add("newPassword", msg)
		return v.OrNil()
	}

	hashed, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}

	if _, err := s.repo.Update(ctx, id, func(u *domain.User) {
		u.Password = hashed
	}); err != nil {
		return err
	}

	// Every existing session was authorized by the old password. Changing it is
	// how someone locks out whoever they think is in their account, so the
	// sessions have to go with it.
	s.revokeSessions(ctx, id, "password changed")
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// The row is soft-deleted, so nothing cascades. Sessions have to be ended
	// explicitly or a deleted user keeps refreshing.
	s.revokeSessions(ctx, id, "user deleted")
	return nil
}

// revokeSessions is best-effort: the change the caller asked for has already
// been committed, so a failure here is logged rather than returned. Access
// tokens are short-lived, which bounds how long a missed revoke matters.
func (s *service) revokeSessions(ctx context.Context, userID uuid.UUID, reason string) {
	if s.sessions == nil {
		return
	}

	revoked, err := s.sessions.RevokeAllForUser(ctx, userID)
	switch {
	case err != nil && !errors.Is(err, domain.ErrNotFound):
		log.Printf("[user] could not revoke sessions for %s after %s: %v", userID, reason, err)
	case revoked > 0:
		log.Printf("[user] revoked %d session(s) for %s: %s", revoked, userID, reason)
	}
}
