package response

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go-commerce/domain"
)

// Machine-readable error codes. A client branches on these rather than on the
// message, which is free to change.
const (
	CodeBadRequest      = "bad_request"
	CodeValidation      = "validation_failed"
	CodeUnauthorized    = "unauthorized"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeConflict        = "conflict"
	CodeUnsupportedType = "unsupported_media_type"
	CodeTooLarge        = "payload_too_large"
	CodeInternal        = "internal_error"
	CodeUnavailable     = "service_unavailable"
)

// Error replies with a failure body.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, errorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// ValidationFailed replies with the per-field messages a validator produced.
func ValidationFailed(w http.ResponseWriter, fields map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, errorEnvelope{Error: ErrorBody{
		Code:    CodeValidation,
		Message: "the request body did not pass validation",
		Fields:  fields,
	}})
}

// Fail turns any error from a service into the right reply.
//
// This is the single translation point between the domain's vocabulary of
// failure and HTTP's. Handlers call it and return; none of them decides a status
// code, which is what stops the same condition from being a 404 in one handler
// and a 400 in the next.
//
// An error it does not recognise becomes a 500 with a generic message, and the
// real error is logged. That asymmetry is deliberate: a database error's text can
// name tables, columns and constraints, and none of that belongs in a reply to
// whoever sent the request.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	// A cancelled request has nobody left to reply to. Writing a 500 for it
	// would log a server fault every time a client closes a connection.
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		return
	}

	// Validation comes first: it is the most specific case and carries the
	// per-field detail that would be lost by the ErrInvalid branch below.
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		ValidationFailed(w, validation.Fields)
		return
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		Error(w, http.StatusNotFound, CodeNotFound, message(err))

	case errors.Is(err, domain.ErrConflict):
		Error(w, http.StatusConflict, CodeConflict, message(err))

	case errors.Is(err, domain.ErrForbidden):
		Error(w, http.StatusForbidden, CodeForbidden, message(err))

	case errors.Is(err, domain.ErrUnauthorized):
		// The header is what tells a client how to authenticate, and RFC 9110
		// requires it on a 401. Without it a browser or SDK has to guess.
		w.Header().Set("WWW-Authenticate", `Bearer realm="go-commerce"`)
		Error(w, http.StatusUnauthorized, CodeUnauthorized, message(err))

	case errors.Is(err, domain.ErrInvalid):
		Error(w, http.StatusBadRequest, CodeBadRequest, message(err))

	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("[%s %s] timed out: %v", r.Method, r.URL.Path, err)
		Error(w, http.StatusServiceUnavailable, CodeUnavailable,
			"the request took too long to process")

	default:
		// Logged in full, reported in the vaguest terms the client can act on.
		log.Printf("[%s %s] unhandled error: %v", r.Method, r.URL.Path, err)
		Error(w, http.StatusInternalServerError, CodeInternal,
			"something went wrong handling this request")
	}
}

// message is the text of a domain error, which is written to be shown. The
// sentinel it wraps is already reflected in the status code, so only the leading
// clause is useful to a reader -- but keeping the whole string costs nothing and
// stays honest about what was matched.
func message(err error) string {
	return err.Error()
}
