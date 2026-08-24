// Package migration carries the database schema.
//
// The .sql files are embedded, so a deployed binary can migrate on its own with
// no separate tool to install and no way for the schema to drift from the build
// that expects it: what ships is what runs.
//
// Files are named
//
//	<version>_<name>.up.sql
//	<version>_<name>.down.sql
//
// where version is digits and orders the migration. The first three are 0001,
// 0002, 0003; anything generated after this is stamped with a UTC timestamp
// (20060102150405), which sorts after them and cannot collide with a version
// somebody else added on another branch.
//
// A down file is optional. Without one the migration simply cannot be rolled
// back, which is the honest answer for a change that destroys data.
package migration

import "embed"

// FS holds every .sql file in this directory.
//
// Generating a migration writes to the directory on disk; it only becomes part
// of this FS on the next build. That is inherent to embedding, and it is the
// same property that makes a released binary self-contained.
//
//go:embed *.sql
var FS embed.FS

// Dir is where the .sql files live in the source tree, which is what the
// generator writes to. It is relative to the module root.
const Dir = "migration"
