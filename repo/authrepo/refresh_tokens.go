package authrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go-commerce/auth"
	"go-commerce/domain"
	"go-commerce/infra/db/dbquery"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RefreshTokenRepo struct {
	dbCon *sqlx.DB
}

// As in userrepo, only this aggregate's own port is asserted. This adapter also
// satisfies user.SessionRevoker -- how a role change or a password change ends
// the sessions it invalidates -- and that assertion lives in cmd rather than
// pulling the user aggregate in here.
var _ auth.TokenStore = (*RefreshTokenRepo)(nil)

func NewRefreshTokenRepo(dbCon *sqlx.DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{dbCon: dbCon}
}

const refreshTokenColumns = `id, user_id, family_id, token_hash, user_agent,
	ip_address, expires_at, revoked_at, replaced_by, created_at`

func (r *RefreshTokenRepo) Create(ctx context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	return insertRefreshToken(ctx, r.dbCon, t)
}

// insertRefreshToken writes one row. It may assign the family id, so it works on
// a copy: filling in the caller's struct would leave the service holding a
// record that looks stored but is not.
func insertRefreshToken(ctx context.Context, q dbquery.Getter, in *domain.RefreshToken) (*domain.RefreshToken, error) {
	if in == nil {
		return nil, fmt.Errorf("storing refresh token: %w", domain.ErrInvalid)
	}

	t := *in
	if t.FamilyID == uuid.Nil {
		// A token with no family is the first of one.
		t.FamilyID = uuid.New()
	}

	query := fmt.Sprintf(`
		INSERT INTO refresh_tokens
			(user_id, family_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING %s`, refreshTokenColumns)

	var created domain.RefreshToken
	if err := q.GetContext(ctx, &created, query,
		t.UserID, t.FamilyID, t.TokenHash, t.UserAgent, t.IPAddress, t.ExpiresAt); err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}
	return &created, nil
}

func (r *RefreshTokenRepo) ByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	query := fmt.Sprintf(`SELECT %s FROM refresh_tokens WHERE token_hash = $1`, refreshTokenColumns)

	var found domain.RefreshToken
	if err := r.dbCon.GetContext(ctx, &found, query, domain.HashToken(token)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("fetching refresh token: %w", err)
	}
	return &found, nil
}

func (r *RefreshTokenRepo) FamilyOf(ctx context.Context, token string) (uuid.UUID, error) {
	var familyID uuid.UUID
	err := r.dbCon.GetContext(ctx, &familyID,
		`SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, domain.HashToken(token))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, domain.ErrRefreshTokenNotFound
		}
		return uuid.Nil, fmt.Errorf("fetching refresh token family: %w", err)
	}
	return familyID, nil
}

// Rotate spends the presented token and stores its successor in one
// transaction.
//
// The revoke is conditional -- `revoked_at IS NULL AND expires_at > now()` --
// and it is that condition, not a preceding SELECT, that detects reuse. Two
// requests arriving with the same token both attempt the UPDATE; exactly one of
// them changes a row, and the other is told the token was already spent. A
// check-then-act would let both through and mint two live sessions from one
// token.
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldToken string, next *domain.RefreshToken) (*domain.RefreshToken, error) {
	if next == nil {
		return nil, fmt.Errorf("rotating refresh token: %w", domain.ErrInvalid)
	}

	tx, err := r.dbCon.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rotating refresh token: %w", err)
	}
	defer tx.Rollback()

	spend := fmt.Sprintf(`
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING %s`, refreshTokenColumns)

	var spent domain.RefreshToken
	if err := tx.GetContext(ctx, &spent, spend, domain.HashToken(oldToken)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, classifyUnspendable(ctx, tx, oldToken)
		}
		return nil, fmt.Errorf("spending refresh token: %w", err)
	}

	// The successor inherits the family and the owner from the row it replaces,
	// not from what the caller passed: the database is what knows whose token
	// this actually was. Written onto a copy, so a rotation that fails to commit
	// does not leave the caller's record claiming a family it never joined.
	successor := *next
	successor.FamilyID = spent.FamilyID
	successor.UserID = spent.UserID

	created, err := insertRefreshToken(ctx, tx, &successor)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET replaced_by = $1 WHERE id = $2`, created.ID, spent.ID); err != nil {
		return nil, fmt.Errorf("linking rotated refresh token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing refresh token rotation: %w", err)
	}
	return created, nil
}

// classifyUnspendable explains why the conditional spend matched nothing, so the
// service can tell an ordinary expired session -- sign in again -- from a
// replayed one, which means a token leaked and the family has to go.
func classifyUnspendable(ctx context.Context, tx *sqlx.Tx, token string) error {
	query := fmt.Sprintf(`SELECT %s FROM refresh_tokens WHERE token_hash = $1`, refreshTokenColumns)

	var existing domain.RefreshToken
	if err := tx.GetContext(ctx, &existing, query, domain.HashToken(token)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRefreshTokenNotFound
		}
		return fmt.Errorf("inspecting refresh token: %w", err)
	}

	if existing.RevokedAt != nil {
		return domain.ErrRefreshTokenReused
	}
	return domain.ErrRefreshTokenExpired
}

func (r *RefreshTokenRepo) RevokeByToken(ctx context.Context, token string) error {
	return r.revokeOne(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL`, domain.HashToken(token))
}

// RevokeByID ends one session by id. The user id is part of the condition, so a
// client can only end its own sessions whatever id it guesses -- the check is in
// the statement rather than in a preceding read that a caller could skip.
func (r *RefreshTokenRepo) RevokeByID(ctx context.Context, id, userID uuid.UUID) error {
	return r.revokeOne(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, userID)
}

func (r *RefreshTokenRepo) revokeOne(ctx context.Context, query string, args ...any) error {
	res, err := r.dbCon.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking revoke result: %w", err)
	}
	if rows == 0 {
		return domain.ErrRefreshTokenNotFound
	}
	return nil
}

func (r *RefreshTokenRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID) (int64, error) {
	return r.revokeMany(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE family_id = $1 AND revoked_at IS NULL`, familyID)
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.revokeMany(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
}

func (r *RefreshTokenRepo) revokeMany(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := r.dbCon.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("revoking refresh tokens: %w", err)
	}
	return res.RowsAffected()
}

// ActiveByUser lists the sessions a user could still refresh, newest first --
// what a "where you are signed in" screen shows.
func (r *RefreshTokenRepo) ActiveByUser(ctx context.Context, userID uuid.UUID, page domain.Page) (*domain.PageResult[*domain.RefreshToken], error) {
	const activeWhere = `WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()`

	query := fmt.Sprintf(`
		SELECT %s, COUNT(*) OVER () AS total_count
		FROM refresh_tokens
		%s
		ORDER BY created_at DESC, id ASC
		LIMIT $2 OFFSET $3`, refreshTokenColumns, activeWhere)

	var rows []struct {
		domain.RefreshToken
		TotalCount int `db:"total_count"`
	}
	if err := r.dbCon.SelectContext(ctx, &rows, query, userID, page.Limit(), page.Offset()); err != nil {
		return nil, fmt.Errorf("listing sessions for %s: %w", userID, err)
	}

	items := make([]*domain.RefreshToken, 0, len(rows))
	total := 0
	for _, row := range rows {
		found := row.RefreshToken
		items = append(items, &found)
		total = row.TotalCount
	}

	if len(rows) == 0 {
		var err error
		if total, err = dbquery.CountRows(ctx, r.dbCon, "refresh_tokens", activeWhere, []any{userID}); err != nil {
			return nil, err
		}
	}

	return &domain.PageResult[*domain.RefreshToken]{Items: items, Total: total, Page: page}, nil
}

// DeleteExpired drops rows that are past expiry by more than retention.
//
// The retention window is what keeps reuse detection working: a revoked token is
// the thing a replay is matched against, so deleting it the moment it expires
// would turn a detectable replay into an unremarkable "not found".
func (r *RefreshTokenRepo) DeleteExpired(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-retention)

	res, err := r.dbCon.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting expired refresh tokens: %w", err)
	}
	return res.RowsAffected()
}
