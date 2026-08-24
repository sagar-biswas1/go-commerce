package cmd

import (
	"go-commerce/auth"
	"go-commerce/repo/authrepo"
	"go-commerce/repo/userrepo"
	"go-commerce/user"
)

// The cross-aggregate wiring contract.
//
// Each adapter asserts its own aggregate's port next to its own code. These are
// the assertions that span two aggregates, and they live here because cmd is the
// only package entitled to know how the pieces fit: an adapter asserting a
// sibling aggregate's port would have to import that aggregate, which is the
// coupling the layout is arranged to prevent.
//
// Go satisfies interfaces structurally, so no adapter needs the import -- but
// without these lines a port change would surface as a confusing argument-type
// error inside Serve rather than as a plain statement of which contract broke.
var (
	// The auth service reads users through a narrow slice of the user adapter:
	// it may look one up and record a login, and cannot delete one.
	_ auth.UserReader = (*userrepo.UserRepo)(nil)

	// The user service ends sessions through the token adapter, so a suspension,
	// a role change or a new password takes effect now rather than whenever the
	// last access token happens to lapse.
	_ user.SessionRevoker = (*authrepo.RefreshTokenRepo)(nil)
)
