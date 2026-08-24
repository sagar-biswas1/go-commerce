package middlewares

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go-commerce/domain"
	"go-commerce/rest/response"
)

// identityKey is the context key the authenticated caller is stored under.
//
// It is an unexported type with an unexported value, which is what makes the
// key unforgeable: no other package can construct this key, so nothing outside
// this file can put an identity into a request context and have IdentityFrom
// find it. A plain string key would be writable by anyone who guessed it.
type contextKey struct{ name string }

var identityKey = &contextKey{"identity"}

// RequireAuth rejects a request that does not carry a valid access token, and
// attaches the caller's identity for the handler behind it.
//
// The Authorization header is the only place it looks. Accepting a token from a
// query string would write it into every access log and browser history along
// the way, and accepting one from a cookie would make every authenticated
// endpoint vulnerable to CSRF -- a browser sends cookies on a cross-site request
// but never an Authorization header.
func (m *Middlewares) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			unauthorized(w, err.Error())
			return
		}

		if m.validator == nil {
			// Refusing is the only safe answer: a missing validator means every
			// request would otherwise be let through unauthenticated.
			response.Error(w, http.StatusInternalServerError, response.CodeInternal,
				"authentication is not configured")
			return
		}

		identity, err := m.validator.ParseAccessToken(token)
		if err != nil {
			unauthorized(w, "the access token is invalid or has expired")
			return
		}

		// The optional live check. It costs a query per request and is what makes
		// a suspension take effect before the token expires.
		if m.users != nil {
			current, err := m.users.ByID(r.Context(), identity.UserID)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					unauthorized(w, "the account this token belongs to no longer exists")
					return
				}
				response.Fail(w, r, err)
				return
			}
			if !current.CanAuthenticate() {
				response.Error(w, http.StatusForbidden, response.CodeForbidden,
					"this account is not active")
				return
			}
			// Trust the stored role over the token's: a role changed a minute ago
			// should not be honoured for the life of a token minted before it.
			identity.Role = current.Role
		}

		next(w, r.WithContext(WithIdentity(r.Context(), identity)))
	}
}

// RequireRole rejects an authenticated caller whose role is not in the allowed
// set. It must sit inside RequireAuth, which is what puts the identity in the
// context -- a missing identity is treated as unauthenticated, not as permitted.
func (m *Middlewares) RequireRole(roles ...string) Middleware {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFrom(r.Context())
			if !ok {
				unauthorized(w, "authentication is required")
				return
			}

			if !allowed[identity.Role] {
				response.Error(w, http.StatusForbidden, response.CodeForbidden,
					"this action requires a different role")
				return
			}

			next(w, r)
		}
	}
}

// RequireAdmin is the common case of RequireRole.
func (m *Middlewares) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return m.RequireRole(domain.RoleAdmin)(next)
}

// bearerToken pulls the credential out of the Authorization header, checking the
// scheme case-insensitively as RFC 9110 requires.
func bearerToken(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", errors.New("an Authorization header is required")
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", errors.New(`the Authorization header must read "Bearer <token>"`)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("the bearer token is empty")
	}
	return token, nil
}

// unauthorized replies 401 with the header that tells a client how to
// authenticate. RFC 9110 requires it on every 401, and without it an SDK has to
// guess the scheme.
func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="go-commerce"`)
	response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, message)
}

// WithIdentity puts an authenticated caller into a context. Exported so a test
// can build a request that is already authenticated without minting a token.
func WithIdentity(ctx context.Context, identity domain.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// IdentityFrom reads the authenticated caller back out.
func IdentityFrom(ctx context.Context) (domain.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(domain.Identity)
	return identity, ok
}

// MustIdentity is IdentityFrom for a handler that only runs behind RequireAuth,
// where an absent identity is a wiring mistake rather than a client error.
func MustIdentity(ctx context.Context) domain.Identity {
	identity, ok := IdentityFrom(ctx)
	if !ok {
		// A zero identity has the nil UUID and no role, so it can act on
		// nothing and satisfies no RequireRole. Failing closed beats panicking
		// in a request handler.
		return domain.Identity{}
	}
	return identity
}
