// Package dbquery is the SQL plumbing the per-aggregate repositories share:
// filter building, validated sort clauses, and the count fallback for a page
// past the end of a collection.
//
// It sits below every repository and imports none of them -- nor any aggregate,
// nor domain. That is what lets three adapters share this code without becoming
// able to reach each other through it: a shared package that knew about one
// aggregate would hand that dependency to every other package using it.
package dbquery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// uniqueViolation is Postgres' code for a broken unique index.
//
// Reading it is how an insert reports "already taken" without a preceding
// SELECT that two concurrent requests could both pass. The index is the only
// thing that can actually decide, so it is what decides.
const uniqueViolation = "23505"

// IsUniqueViolation reports whether err is a unique index violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr) && string(pgErr.Code) == uniqueViolation
}

// Conditions accumulates a WHERE clause and its arguments together, so a
// placeholder number can never drift out of step with the value it stands for.
//
// Values only ever reach the query as placeholders; the clause text is written
// by the calling repository, never taken from a request.
type Conditions struct {
	clauses []string
	args    []any
}

// Placeholder binds a value and returns the "$n" that refers to it.
//
// The caller interpolates that marker into its clause, which is what lets one
// value be used in several places -- a search term matched against three columns
// -- while still being bound exactly once.
func (c *Conditions) Placeholder(value any) string {
	c.args = append(c.args, value)
	return fmt.Sprintf("$%d", len(c.args))
}

// Where adds a clause, ANDed with the rest.
func (c *Conditions) Where(clause string) {
	c.clauses = append(c.clauses, clause)
}

// SQL renders the clause, or "" when nothing was added.
func (c *Conditions) SQL() string {
	if len(c.clauses) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(c.clauses, " AND ")
}

// Args is the argument list for the filter alone, before pagination is
// appended. The count fallback needs exactly these.
func (c *Conditions) Args() []any { return c.args }

// OrderBy renders an ORDER BY, mapping a requested field onto a column through
// allowed.
//
// A field that is not on the list falls back rather than erroring: a client that
// asks to sort by something the collection does not have gets the default order,
// not a rejected request. Because the column comes from allowed and the fallback
// is a constant in the calling package, no part of the result is client-supplied.
func OrderBy(field string, descending bool, allowed map[string]string, fallback string) string {
	column, ok := allowed[field]
	if !ok {
		return fallback
	}

	direction := "ASC"
	if descending {
		direction = "DESC"
	}
	// The tiebreaker on the primary key is what keeps paging stable: without it,
	// two rows with equal sort values can swap places between two queries and a
	// client sees one of them twice and the other never.
	return fmt.Sprintf("%s %s, id ASC", column, direction)
}

// Getter is the little that CountRows needs, so it works on a pool or inside a
// transaction.
type Getter interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
}

// CountRows is the fallback for a page past the end of a collection.
//
// A listing gets its total from a COUNT(*) OVER () window in the same statement
// as the rows, because a separate query can disagree with them. But a page with
// no rows carries no window value to read, and the client still needs a truthful
// total to know how far it overshot.
func CountRows(ctx context.Context, q Getter, table, where string, args []any) (int, error) {
	statement := "SELECT COUNT(*) FROM " + table
	if where != "" {
		statement += " " + where
	}

	var total int
	if err := q.GetContext(ctx, &total, statement, args...); err != nil {
		return 0, fmt.Errorf("counting %s: %w", table, err)
	}
	return total, nil
}
