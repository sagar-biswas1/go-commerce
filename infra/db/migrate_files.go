package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A migration file is named <version>_<name>.<direction>.sql.
//
// The version is digits and decides the order; the name is for a person reading
// the directory. Parsing rather than trusting the string is what stops
// "10_x.up.sql" from sorting before "9_x.up.sql", which is what a plain
// filename sort would do.
var migrationFilePattern = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

// nameCleaner turns whatever a person typed into a filename-safe name.
var nameCleaner = regexp.MustCompile(`[^a-z0-9]+`)

// versionFormat stamps generated migrations with a UTC timestamp.
//
// Sequential numbers are what cause a merge conflict that compiles: two
// branches both add 0004, both get merged, and now two different migrations
// claim one version. A timestamp cannot collide by accident.
const versionFormat = "20060102150405"

// Migration is one versioned schema change, as found on disk.
type Migration struct {
	Version int64
	Name    string
	UpFile  string
	// DownFile is empty when the migration has no down file, which is how a
	// deliberately irreversible change is expressed.
	DownFile string
}

// Reversible reports whether this migration ships a down file.
func (m Migration) Reversible() bool { return m.DownFile != "" }

// String is what the CLI prints: the version and the human-readable name.
func (m Migration) String() string { return fmt.Sprintf("%d_%s", m.Version, m.Name) }

// loadMigrations reads every migration in fsys, in version order.
//
// It pairs up and down files by version rather than by name, so renaming the
// human-readable half of a filename does not silently orphan its down file.
func loadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("listing migrations: %w", err)
	}

	byVersion := make(map[int64]*Migration, len(entries))
	for _, entry := range entries {
		match := migrationFilePattern.FindStringSubmatch(entry)
		if match == nil {
			return nil, fmt.Errorf(
				"migration %q is not named <version>_<name>.up.sql or <version>_<name>.down.sql", entry)
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration %q has an unreadable version: %w", entry, err)
		}

		found, ok := byVersion[version]
		if !ok {
			found = &Migration{Version: version, Name: match[2]}
			byVersion[version] = found
		}
		if found.Name != match[2] {
			return nil, fmt.Errorf(
				"version %d is claimed by two migrations, %q and %q", version, found.Name, match[2])
		}

		if match[3] == "up" {
			found.UpFile = entry
		} else {
			found.DownFile = entry
		}
	}

	migrations := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.UpFile == "" {
			// A down file with no up file is a half-deleted migration. Running
			// what is left would take the schema somewhere no build describes.
			return nil, fmt.Errorf("migration %d_%s has a down file but no up file", m.Version, m.Name)
		}
		migrations = append(migrations, *m)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

// checksum is what detects a migration edited after it was applied.
//
// Editing an applied migration is how two databases end up claiming the same
// version with different schemas -- the one that ran it before the edit and the
// one that ran it after. Nothing else in the system would ever say so.
func checksum(statements []byte) string {
	sum := sha256.Sum256(statements)
	return hex.EncodeToString(sum[:])
}

// Generate writes an empty up/down pair into dir and returns their paths.
//
// The version is a UTC timestamp, so two people generating on two branches at
// the same minute is the worst case rather than the normal one.
func Generate(dir, name string) (upPath, downPath string, err error) {
	clean := nameCleaner.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "_")
	clean = strings.Trim(clean, "_")
	if clean == "" {
		return "", "", fmt.Errorf("a migration needs a name, such as \"add orders table\"")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return "", "", fmt.Errorf("migration directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("migration directory %s is not a directory", dir)
	}

	version := time.Now().UTC().Format(versionFormat)
	upPath = filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", version, clean))
	downPath = filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", version, clean))

	// Refuse rather than overwrite: the same name generated twice in one second
	// would otherwise silently replace work that is already there.
	for _, path := range []string{upPath, downPath} {
		if _, err := os.Stat(path); err == nil {
			return "", "", fmt.Errorf("%s already exists", path)
		}
	}

	if err := os.WriteFile(upPath, []byte(upTemplate(clean)), 0o644); err != nil {
		return "", "", fmt.Errorf("writing %s: %w", upPath, err)
	}
	if err := os.WriteFile(downPath, []byte(downTemplate(clean)), 0o644); err != nil {
		return "", "", fmt.Errorf("writing %s: %w", downPath, err)
	}

	return upPath, downPath, nil
}

func upTemplate(name string) string {
	return fmt.Sprintf(`-- %s
--
-- Runs inside a transaction, so this either lands whole or not at all.
-- Postgres has transactional DDL, which is why there is no half-applied state
-- to clean up by hand.

`, name)
}

func downTemplate(name string) string {
	return fmt.Sprintf(`-- Reverses %s.
--
-- Delete this file instead of leaving it empty if the change cannot be undone:
-- an empty down file claims the rollback worked while changing nothing, and a
-- missing one says plainly that there is no way back.

`, name)
}
