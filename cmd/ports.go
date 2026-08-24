package cmd

import (
	"go-commerce/auth"
	"go-commerce/product"
	"go-commerce/repo/authrepo"
	"go-commerce/repo/userrepo"
	authhandler "go-commerce/rest/handlers/auth"
	producthandler "go-commerce/rest/handlers/product"
	userhandler "go-commerce/rest/handlers/user"
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

	// The transport declares the service shape it drives, and the aggregate
	// declares the shape it offers. Neither imports the other, so this is where
	// the two are checked against each other -- otherwise the day they drift the
	// error would land on the NewHandler line, blaming the wiring for a change
	// made in a port.
	_ producthandler.Service = (product.Service)(nil)
	_ userhandler.Service    = (user.Service)(nil)
	_ authhandler.Service    = (auth.Service)(nil)
)
