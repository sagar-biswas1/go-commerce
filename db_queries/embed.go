// Package dbqueries carries the SQL schema as embedded files so the binary can
// migrate a database on its own, with no separate migration tool to install and
// no risk of the schema drifting from the build that expects it.
package dbqueries

import "embed"

// Migrations holds every *.sql file in this directory. They are applied in
// filename order, so the numeric prefix is the version.
//
//go:embed *.sql
var Migrations embed.FS
