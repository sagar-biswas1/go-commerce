// Package helpers holds the small request-reading utilities the handler packages
// share. It reports errors rather than writing to the ResponseWriter, so each
// module can phrase its own reply without this package knowing any of them.
package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// MaxBodyBytes caps a request body. Without a limit, a single request can ask
// the process to allocate as much memory as the sender is willing to send.
const MaxBodyBytes = 1 << 20 // 1 MiB

// PathUUID reads a UUID wildcard such as {id} out of the request path.
func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := r.PathValue(name)

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s %q is not a valid id: %w", name, raw, domain.ErrInvalid)
	}
	return id, nil
}

// DecodeJSON reads a JSON body into target.
//
// It does three things a bare json.Decoder does not, each closing a way a
// malformed request could be misread:
//
//   - caps the body, so a huge payload cannot exhaust memory;
//   - rejects unknown fields, so a client that misspells "price" is told so
//     instead of silently having the field ignored;
//   - rejects trailing content, so a body holding two JSON objects cannot have
//     its second half quietly discarded.
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return decodeError(err)
	}

	// Decode stops at the end of the first value; anything after it was never
	// looked at.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("body must contain a single JSON object: %w", domain.ErrInvalid)
	}

	return nil
}

// decodeError turns a decoder failure into a message worth showing. The
// library's own text names Go types and byte offsets, which tell a client
// nothing about the JSON it sent.
func decodeError(err error) error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		tooLarge  *http.MaxBytesError
	)

	switch {
	case errors.As(err, &tooLarge):
		return fmt.Errorf("request body must be %d bytes or fewer: %w", MaxBodyBytes, domain.ErrInvalid)

	case errors.As(err, &syntaxErr):
		return fmt.Errorf("body is not valid JSON (at position %d): %w", syntaxErr.Offset, domain.ErrInvalid)

	case errors.As(err, &typeErr):
		return fmt.Errorf("field %q must be a %s: %w", typeErr.Field, typeErr.Type, domain.ErrInvalid)

	case errors.Is(err, io.EOF):
		return fmt.Errorf("a JSON body is required: %w", domain.ErrInvalid)

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf("unknown field %s: %w", field, domain.ErrInvalid)

	default:
		return fmt.Errorf("body could not be read as JSON: %w", domain.ErrInvalid)
	}
}

// MaxUserAgentLen bounds a recorded user agent. The header is
// attacker-controlled and can be arbitrarily long, and it is only ever shown
// back to the user it belongs to.
const MaxUserAgentLen = 500

// UserAgent is the client's self-description, bounded.
//
// It returns a plain string rather than any aggregate's type: this package is
// shared by every handler, so a return type from one aggregate would make every
// other handler depend on it. Assembling that aggregate's own value is the job
// of the handler that needs it.
func UserAgent(r *http.Request) string {
	return truncate(r.UserAgent(), MaxUserAgentLen)
}

// ClientIP is the address the request came from.
//
// X-Forwarded-For is deliberately ignored. Any client can send that header, so
// behind no proxy it is simply a value the caller chose -- and reading it would
// make the recorded address a lie. A deployment that does sit behind a trusted
// proxy should strip and re-set it there, and this is the place to read it from.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
