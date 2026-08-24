package db

import (
	"fmt"
	"io/fs"
	"log"
	"sort"

	"github.com/jmoiron/sqlx"
)

// schemaMigrations records which files have already run. Its own creation is
// idempotent, which is what lets the very first migration be applied to a
// database that has never seen this program.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// Migrate applies every *.sql file in fsys that has not been applied yet, in
// filename order, each inside its own transaction.
//
// It takes an fs.FS rather than a directory path so the schema can travel
// inside the binary -- a deployed build cannot find itself missing the .sql
// files it was compiled against -- while a test can hand it an fstest.MapFS.
func Migrate(dbCon *sqlx.DB, fsys fs.FS) error {
	if dbCon == nil {
		return fmt.Errorf("migrate: no database connection")
	}

	if _, err := dbCon.Exec(schemaMigrationsDDL); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	applied, err := appliedVersions(dbCon)
	if err != nil {
		return err
	}

	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("listing migrations: %w", err)
	}
	// Glob order is not promised to be sorted; the numeric filename prefix is
	// the version, so sorting by name is what puts them in order.
	sort.Strings(files)

	ran := 0
	for _, name := range files {
		if applied[name] {
			continue
		}

		statements, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		if err := applyOne(dbCon, name, string(statements)); err != nil {
			return err
		}

		log.Printf("[migrate] applied %s", name)
		ran++
	}

	if ran == 0 {
		log.Printf("[migrate] schema up to date (%d migrations)", len(files))
	}
	return nil
}

// applyOne runs a single migration and records it in the same transaction, so
// the schema change and the bookkeeping cannot disagree: either both land or
// neither does, and a crash mid-migration leaves the file to be retried.
func applyOne(dbCon *sqlx.DB, version, statements string) error {
	tx, err := dbCon.Beginx()
	if err != nil {
		return fmt.Errorf("beginning migration %s: %w", version, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(statements); err != nil {
		return fmt.Errorf("applying migration %s: %w", version, err)
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("recording migration %s: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %s: %w", version, err)
	}
	return nil
}

func appliedVersions(dbCon *sqlx.DB) (map[string]bool, error) {
	var versions []string
	if err := dbCon.Select(&versions, `SELECT version FROM schema_migrations`); err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}

	applied := make(map[string]bool, len(versions))
	for _, v := range versions {
		applied[v] = true
	}
	return applied, nil
}
