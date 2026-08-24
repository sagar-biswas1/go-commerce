package userrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go-commerce/domain"
	"go-commerce/infra/db/dbquery"
	"go-commerce/user"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepo struct {
	dbCon *sqlx.DB
}

// This adapter answers its own aggregate's port, and only that one.
//
// It also happens to satisfy auth.UserReader, which is how the auth service
// reads users -- but asserting that here would mean importing the auth
// aggregate, and a user-side adapter reaching into a sibling aggregate is
// exactly the coupling this layout exists to prevent. Go satisfies interfaces
// structurally, so no import is needed: the cross-aggregate assertions live in
// cmd, the one package allowed to know how everything fits together.
var _ user.Repository = (*UserRepo)(nil)

func NewUserRepo(dbCon *sqlx.DB) *UserRepo {
	return &UserRepo{dbCon: dbCon}
}

const userColumns = `id, email, password_hash, first_name, last_name, avatar_url,
	phone_number, role, status, is_email_verified, last_login_at,
	created_at, updated_at, deleted_at`

var userSortFields = map[string]string{
	"createdAt": "created_at",
	"updatedAt": "updated_at",
	"email":     "lower(email)",
	"lastName":  "lower(last_name)",
}

const userDefaultOrder = "created_at DESC, id ASC"

func (r *UserRepo) All(ctx context.Context, page domain.Page, filter *domain.UserFilter) (*domain.PageResult[*domain.User], error) {
	if filter == nil {
		filter = &domain.UserFilter{}
	}

	conds := userConditions(filter)
	where := conds.SQL()
	args := conds.Args()

	query := fmt.Sprintf(`
		SELECT %s, COUNT(*) OVER () AS total_count
		FROM users
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		userColumns,
		where,
		dbquery.OrderBy(filter.Sort.Field, filter.Sort.Descending, userSortFields, userDefaultOrder),
		len(args)+1, len(args)+2,
	)

	var rows []struct {
		domain.User
		TotalCount int `db:"total_count"`
	}
	if err := r.dbCon.SelectContext(ctx, &rows, query, append(args, page.Limit(), page.Offset())...); err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	items := make([]*domain.User, 0, len(rows))
	total := 0
	for _, row := range rows {
		// Copied out explicitly, so what escapes to the caller is a user and not
		// a pointer into a scan row that also carries the count.
		found := row.User
		items = append(items, &found)
		total = row.TotalCount
	}

	// An empty page still has to report the size of the collection: without a
	// separate count, page 5 of a 3-page result would claim there are no users.
	if len(rows) == 0 {
		var err error
		if total, err = dbquery.CountRows(ctx, r.dbCon, "users", where, args); err != nil {
			return nil, err
		}
	}

	return &domain.PageResult[*domain.User]{Items: items, Total: total, Page: page}, nil
}

// userConditions always excludes soft-deleted rows. A deleted user is gone as
// far as every caller of this adapter is concerned, and making that the default
// here means no caller can forget it.
func userConditions(filter *domain.UserFilter) *dbquery.Conditions {
	conds := &dbquery.Conditions{}
	conds.Where("deleted_at IS NULL")

	if search := strings.TrimSpace(filter.Search); search != "" {
		marker := conds.Placeholder("%" + strings.ToLower(search) + "%")
		conds.Where(fmt.Sprintf(
			"(lower(email) LIKE %s OR lower(first_name) LIKE %s OR lower(last_name) LIKE %s)",
			marker, marker, marker))
	}
	if filter.Role != "" {
		conds.Where("role = " + conds.Placeholder(filter.Role))
	}
	if filter.Status != "" {
		conds.Where("status = " + conds.Placeholder(filter.Status))
	}

	return conds
}

func (r *UserRepo) ByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM users WHERE id = $1 AND deleted_at IS NULL`, userColumns)

	var found domain.User
	if err := r.dbCon.GetContext(ctx, &found, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("fetching user %s: %w", id, err)
	}
	return &found, nil
}

// ByEmail matches on the lowered address, which is what the unique index is
// built on -- so a lookup can never miss a row that the index would refuse to
// let be inserted twice.
func (r *UserRepo) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM users WHERE lower(email) = $1 AND deleted_at IS NULL`, userColumns)

	var found domain.User
	if err := r.dbCon.GetContext(ctx, &found, query, domain.NormalizeEmail(email)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("fetching user by email: %w", err)
	}
	return &found, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	query := fmt.Sprintf(`
		INSERT INTO users (email, password_hash, first_name, last_name,
		                   avatar_url, phone_number, role, status, is_email_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING %s`, userColumns)

	var created domain.User
	err := r.dbCon.GetContext(ctx, &created, query,
		domain.NormalizeEmail(u.Email), u.Password, u.FirstName, u.LastName,
		u.AvatarURL, u.PhoneNumber, u.Role, u.Status, u.IsEmailVerified)
	if err != nil {
		if dbquery.IsUniqueViolation(err) {
			return nil, domain.ErrEmailTaken
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return &created, nil
}

// Update locks the row for the read-modify-write, for the same reason
// productRepo.Update does.
func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, apply func(*domain.User)) (*domain.User, error) {
	tx, err := r.dbCon.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("updating user %s: %w", id, err)
	}
	defer tx.Rollback()

	var current domain.User
	lockRow := fmt.Sprintf(
		`SELECT %s FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userColumns)
	if err := tx.GetContext(ctx, &current, lockRow, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("locking user %s: %w", id, err)
	}

	apply(&current)

	query := fmt.Sprintf(`
		UPDATE users
		SET email = $1, password_hash = $2, first_name = $3, last_name = $4,
		    avatar_url = $5, phone_number = $6, role = $7, status = $8,
		    is_email_verified = $9, updated_at = now()
		WHERE id = $10
		RETURNING %s`, userColumns)

	var updated domain.User
	err = tx.GetContext(ctx, &updated, query,
		domain.NormalizeEmail(current.Email), current.Password, current.FirstName, current.LastName,
		current.AvatarURL, current.PhoneNumber, current.Role, current.Status,
		current.IsEmailVerified, id)
	if err != nil {
		if dbquery.IsUniqueViolation(err) {
			return nil, domain.ErrEmailTaken
		}
		return nil, fmt.Errorf("updating user %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing user %s: %w", id, err)
	}
	return &updated, nil
}

// Delete is a soft delete. Orders, sessions and audit history reference a user,
// and removing the row would take that history with it; the partial unique index
// on lower(email) means the freed address can still be registered again.
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.dbCon.ExecContext(ctx, `
		UPDATE users
		SET deleted_at = now(), updated_at = now(), status = $2
		WHERE id = $1 AND deleted_at IS NULL`, id, domain.StatusInactive)
	if err != nil {
		return fmt.Errorf("deleting user %s: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result for %s: %w", id, err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	if _, err := r.dbCon.ExecContext(ctx,
		`UPDATE users SET last_login_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("recording login for %s: %w", id, err)
	}
	return nil
}
