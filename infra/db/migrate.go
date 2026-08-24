package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// schemaMigrationsDDL records what has run. Its own creation is idempotent,
// which is what lets the very first migration be applied to a database that has
// never seen this program.
//
// The checksum column is what makes an edited-after-apply migration detectable.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    BIGINT PRIMARY KEY,
    name       TEXT NOT NULL,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// advisoryLockID serialises migration runs across processes.
//
// Two instances of a deployed service starting together would otherwise both
// find the same work pending and both try to do it. The value is arbitrary but
// has to stay fixed: it is only a name that two processes agree on.
const advisoryLockID int64 = 4_819_207_331

// lockTimeout bounds how long to wait for another process to finish migrating.
// Without it, a stuck migration holds every other instance at the door forever.
const lockTimeout = 2 * time.Minute

// AppliedMigration is one row of schema_migrations.
type AppliedMigration struct {
	Version   int64     `db:"version"`
	Name      string    `db:"name"`
	Checksum  string    `db:"checksum"`
	AppliedAt time.Time `db:"applied_at"`
}

// Status is what one migration looks like to a person asking.
type Status struct {
	Migration Migration
	Applied   *AppliedMigration
	// Modified means the file on disk no longer matches what was applied.
	Modified bool
}

func (s Status) State() string {
	switch {
	case s.Applied == nil:
		return "pending"
	case s.Modified:
		return "MODIFIED"
	default:
		return "applied"
	}
}

// Runner applies and reverses migrations.
//
// It takes an fs.FS rather than a directory path so the schema can travel
// inside the binary -- a deployed build cannot find itself missing the .sql
// files it was compiled against -- while a test can hand it an fstest.MapFS.
type Runner struct {
	dbCon *sqlx.DB
	files fs.FS
}

func NewRunner(dbCon *sqlx.DB, files fs.FS) *Runner {
	return &Runner{dbCon: dbCon, files: files}
}

// Up applies every pending migration in version order, each in its own
// transaction. A limit of 0 means "all of them".
func (r *Runner) Up(ctx context.Context, limit int) ([]Migration, error) {
	return r.run(ctx, func(ctx context.Context) ([]Migration, error) {
		migrations, applied, err := r.load(ctx)
		if err != nil {
			return nil, err
		}

		var ran []Migration
		for _, m := range migrations {
			if _, done := applied[m.Version]; done {
				continue
			}
			if limit > 0 && len(ran) == limit {
				break
			}

			statements, err := fs.ReadFile(r.files, m.UpFile)
			if err != nil {
				return ran, fmt.Errorf("reading %s: %w", m.UpFile, err)
			}

			if err := r.apply(ctx, m, string(statements), checksum(statements)); err != nil {
				return ran, err
			}
			log.Printf("[migrate] applied %s", m)
			ran = append(ran, m)
		}
		return ran, nil
	})
}

// Down rolls back the most recently applied migrations, newest first.
//
// A count of 0 is treated as 1: rolling back everything by default is how a
// mistyped command empties a database.
func (r *Runner) Down(ctx context.Context, count int) ([]Migration, error) {
	if count < 1 {
		count = 1
	}

	return r.run(ctx, func(ctx context.Context) ([]Migration, error) {
		migrations, applied, err := r.load(ctx)
		if err != nil {
			return nil, err
		}

		var rolled []Migration
		// Newest first, which is the reverse of the order they went up. A
		// foreign key added by a later migration would otherwise block the
		// table an earlier one created.
		for i := len(migrations) - 1; i >= 0 && len(rolled) < count; i-- {
			m := migrations[i]
			if _, done := applied[m.Version]; !done {
				continue
			}
			if !m.Reversible() {
				return rolled, fmt.Errorf(
					"%s has no down file, so it cannot be rolled back", m)
			}

			statements, err := fs.ReadFile(r.files, m.DownFile)
			if err != nil {
				return rolled, fmt.Errorf("reading %s: %w", m.DownFile, err)
			}

			if err := r.revert(ctx, m, string(statements)); err != nil {
				return rolled, err
			}
			log.Printf("[migrate] rolled back %s", m)
			rolled = append(rolled, m)
		}
		return rolled, nil
	})
}

// Status reports every migration and whether it has run. It takes no lock: it
// changes nothing, and a report that blocks behind a running migration is worse
// than one that describes the moment it was asked.
func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}

	// Deliberately not verified: a report that refuses to render because
	// something is wrong is the report you most need when something is wrong.
	// Drift shows up as a MODIFIED row instead, and it is `up` and `down` that
	// refuse to run.
	migrations, applied, err := r.read(ctx)
	if err != nil {
		return nil, err
	}

	report := make([]Status, 0, len(migrations))
	for _, m := range migrations {
		status := Status{Migration: m}
		if row, ok := applied[m.Version]; ok {
			copied := row
			status.Applied = &copied

			if statements, err := fs.ReadFile(r.files, m.UpFile); err == nil {
				status.Modified = checksum(statements) != row.Checksum
			}
		}
		report = append(report, status)
	}
	return report, nil
}

// run holds the advisory lock for the duration of fn.
//
// The lock is taken on its own connection and released on the same one, because
// a pooled query could otherwise unlock from a connection that never held it.
func (r *Runner) run(ctx context.Context, fn func(context.Context) ([]Migration, error)) ([]Migration, error) {
	if r.dbCon == nil {
		return nil, errors.New("migrate: no database connection")
	}
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}

	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	conn, err := r.dbCon.Connx(lockCtx)
	if err != nil {
		return nil, fmt.Errorf("migrate: reserving a connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(lockCtx, `SELECT pg_advisory_lock($1)`, advisoryLockID); err != nil {
		return nil, fmt.Errorf("migrate: another process is already migrating: %w", err)
	}
	defer func() {
		// A fresh context: the caller's may already be cancelled, and failing to
		// release would leave every other instance waiting out the timeout.
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer unlockCancel()

		if _, err := conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, advisoryLockID); err != nil {
			log.Printf("[migrate] could not release the migration lock: %v", err)
		}
	}()

	return fn(ctx)
}

func (r *Runner) ensureTable(ctx context.Context) error {
	if r.dbCon == nil {
		return errors.New("migrate: no database connection")
	}

	upgraded, err := r.adoptLegacyBookkeeping(ctx)
	if err != nil {
		return err
	}
	if upgraded {
		return nil
	}

	if _, err := r.dbCon.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}
	return nil
}

// legacyVersionPrefix reads the leading digits of an old-style version string,
// which was the migration's whole filename: "003-refresh-tokens.sql" -> 3.
var legacyVersionPrefix = regexp.MustCompile(`^(\d+)`)

// adoptLegacyBookkeeping upgrades a schema_migrations table written by the
// earlier runner, which keyed rows on the filename and kept no checksum.
//
// Renaming the .sql files would otherwise orphan every applied row: the new
// runner would find three migrations pending against a database that already has
// all three tables, and re-run them. They are written with IF NOT EXISTS so that
// would not corrupt anything, but the history would be a fiction from then on.
//
// The old table is renamed aside rather than dropped. It is the only record of
// when the schema was first applied, and this is not the place to decide that
// nobody wants it.
func (r *Runner) adoptLegacyBookkeeping(ctx context.Context) (bool, error) {
	var legacy bool
	if err := r.dbCon.GetContext(ctx, &legacy, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = 'schema_migrations'
		) AND NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'schema_migrations' AND column_name = 'checksum'
		)`); err != nil {
		return false, fmt.Errorf("inspecting schema_migrations: %w", err)
	}
	if !legacy {
		return false, nil
	}

	migrations, err := loadMigrations(r.files)
	if err != nil {
		return false, err
	}
	byVersion := make(map[int64]Migration, len(migrations))
	for _, m := range migrations {
		byVersion[m.Version] = m
	}

	var oldVersions []string
	if err := r.dbCon.SelectContext(ctx, &oldVersions,
		`SELECT version FROM schema_migrations ORDER BY version`); err != nil {
		return false, fmt.Errorf("reading the previous migration history: %w", err)
	}

	// Translate before writing anything, so an entry that cannot be matched
	// leaves the old table exactly as it was.
	type adopted struct {
		legacyVersion string
		version       int64
		name          string
		checksum      string
	}
	rows := make([]adopted, 0, len(oldVersions))

	for _, old := range oldVersions {
		match := legacyVersionPrefix.FindStringSubmatch(old)
		if match == nil {
			return false, fmt.Errorf(
				"the previous migration history contains %q, which has no version number to carry over", old)
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return false, fmt.Errorf("unreadable previous version %q: %w", old, err)
		}

		m, ok := byVersion[version]
		if !ok {
			return false, fmt.Errorf(
				"the database has %q applied, but this build has no migration numbered %d", old, version)
		}

		statements, err := fs.ReadFile(r.files, m.UpFile)
		if err != nil {
			return false, fmt.Errorf("reading %s: %w", m.UpFile, err)
		}
		rows = append(rows, adopted{
			legacyVersion: old,
			version:       version,
			name:          m.Name,
			checksum:      checksum(statements),
		})
	}

	tx, err := r.dbCon.BeginTxx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("upgrading schema_migrations: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`ALTER TABLE schema_migrations RENAME TO schema_migrations_legacy`); err != nil {
		return false, fmt.Errorf("setting the previous history aside: %w", err)
	}
	if _, err := tx.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return false, fmt.Errorf("creating schema_migrations: %w", err)
	}

	for _, row := range rows {
		// The original applied_at is carried across, matched on the exact old
		// version string: a LIKE on the number would let "1" claim the row for
		// "0010_...".
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schema_migrations (version, name, checksum, applied_at)
			SELECT $1, $2, $3, applied_at
			FROM schema_migrations_legacy
			WHERE version = $4`,
			row.version, row.name, row.checksum, row.legacyVersion); err != nil {
			return false, fmt.Errorf("carrying over migration %d: %w", row.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("committing the schema_migrations upgrade: %w", err)
	}

	log.Printf("[migrate] carried %d migration(s) over from the previous history; "+
		"the old table is kept as schema_migrations_legacy", len(rows))
	return true, nil
}

// read gathers the migrations on disk and the rows recording what has run,
// without judging whether the two agree.
func (r *Runner) read(ctx context.Context) ([]Migration, map[int64]AppliedMigration, error) {
	migrations, err := loadMigrations(r.files)
	if err != nil {
		return nil, nil, err
	}

	var rows []AppliedMigration
	if err := r.dbCon.SelectContext(ctx, &rows,
		`SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`); err != nil {
		return nil, nil, fmt.Errorf("reading applied migrations: %w", err)
	}

	applied := make(map[int64]AppliedMigration, len(rows))
	for _, row := range rows {
		applied[row.Version] = row
	}
	return migrations, applied, nil
}

// load is read plus the checks that have to pass before anything is written.
func (r *Runner) load(ctx context.Context) ([]Migration, map[int64]AppliedMigration, error) {
	migrations, applied, err := r.read(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := verify(migrations, applied, r.files); err != nil {
		return nil, nil, err
	}
	return migrations, applied, nil
}

// verify catches the two ways a schema history can be wrong in a way that no
// later error would explain.
func verify(migrations []Migration, applied map[int64]AppliedMigration, files fs.FS) error {
	known := make(map[int64]Migration, len(migrations))
	for _, m := range migrations {
		known[m.Version] = m
	}

	for version, row := range applied {
		m, ok := known[version]
		if !ok {
			// The database has run something this build does not carry. Applying
			// the rest would leave a schema no version of this program describes.
			return fmt.Errorf(
				"the database has migration %d_%s applied, but this build does not contain it",
				version, row.Name)
		}

		statements, err := fs.ReadFile(files, m.UpFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", m.UpFile, err)
		}
		if got := checksum(statements); got != row.Checksum {
			return fmt.Errorf(
				"%s was changed after it was applied; write a new migration instead of editing an applied one", m)
		}
	}
	return nil
}

// apply runs one migration and records it in the same transaction, so the
// schema change and the bookkeeping cannot disagree: either both land or
// neither does. Postgres has transactional DDL, which is why a crash halfway
// through leaves nothing to repair by hand.
func (r *Runner) apply(ctx context.Context, m Migration, statements, sum string) error {
	return r.inTx(ctx, m, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, statements); err != nil {
			return fmt.Errorf("applying %s: %w", m, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
			m.Version, m.Name, sum); err != nil {
			return fmt.Errorf("recording %s: %w", m, err)
		}
		return nil
	})
}

func (r *Runner) revert(ctx context.Context, m Migration, statements string) error {
	return r.inTx(ctx, m, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, statements); err != nil {
			return fmt.Errorf("rolling back %s: %w", m, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM schema_migrations WHERE version = $1`, m.Version); err != nil {
			return fmt.Errorf("un-recording %s: %w", m, err)
		}
		return nil
	})
}

func (r *Runner) inTx(ctx context.Context, m Migration, fn func(*sqlx.Tx) error) error {
	tx, err := r.dbCon.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning %s: %w", m, err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing %s: %w", m, err)
	}
	return nil
}

// Migrate applies every pending migration. It is what the server calls on
// startup, so a process either serves a schema it agrees with or does not
// start.
func Migrate(ctx context.Context, dbCon *sqlx.DB, files fs.FS) error {
	ran, err := NewRunner(dbCon, files).Up(ctx, 0)
	if err != nil {
		return err
	}
	if len(ran) == 0 {
		log.Print("[migrate] schema up to date")
	}
	return nil
}
