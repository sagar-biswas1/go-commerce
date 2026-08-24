// Package domain holds the entities and the rules that govern them, and depends
// on nothing else in this program. Everything else points inward at it: the
// service packages orchestrate these types, repo stores them, rest exposes them.
//
// That direction is the whole point. A rule such as "a suspended user cannot
// hold a session" lives here once, so it cannot be enforced by the HTTP layer
// and quietly skipped by a background job.
package domain

import "errors"

// The vocabulary of failure. Every layer above matches on these with errors.Is
// rather than on driver errors or sentinel strings, which is what lets the HTTP
// layer map a failure to a status code without knowing there is a database.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("not authenticated")
	ErrForbidden    = errors.New("not permitted")
)

// ValidationError carries one message per rejected field, so a client is told
// everything wrong with its request at once instead of one problem per attempt.
type ValidationError struct {
	Fields map[string]string
}

func NewValidationError() *ValidationError {
	return &ValidationError{Fields: make(map[string]string)}
}

// Add records a rejected field. Nothing is reported until the caller asks, so a
// validator can run every rule before deciding the value is unusable.
func (e *ValidationError) Add(field, message string) {
	e.Fields[field] = message
}

func (e *ValidationError) Any() bool { return len(e.Fields) > 0 }

// OrNil returns nil when nothing was rejected. Returning *ValidationError
// directly would produce a non-nil error interface holding an empty struct,
// which is the classic way a "no errors" result reads as a failure.
func (e *ValidationError) OrNil() error {
	if e.Any() {
		return e
	}
	return nil
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

// Unwrap makes every validation failure match ErrInvalid, so a handler can map
// the whole family to 422 with one errors.Is.
func (e *ValidationError) Unwrap() error { return ErrInvalid }
