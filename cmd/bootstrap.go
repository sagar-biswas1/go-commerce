package cmd

import (
	"context"
	"errors"
	"log"

	"go-commerce/config"
	"go-commerce/domain"
	"go-commerce/user"
)

// bootstrapAdmin creates the first administrator when the database has none.
//
// A fresh deployment is otherwise unusable: public registration only ever makes
// an ordinary user, and POST /users needs an admin to call it, so there is no
// first move. This closes that gap without seeding a fixed account into a
// migration -- a hardcoded password in version control is a password everyone
// who reads the repository knows.
//
// It runs only when the users table has no admin, so restarting the process does
// not keep recreating or resetting one.
func bootstrapAdmin(ctx context.Context, cfg *config.Config, service user.Service, repository user.Repository) {
	// One row is enough to answer "is there an admin", so ask for one.
	existing, err := repository.All(ctx, domain.NewPage(1, 1), &domain.UserFilter{Role: domain.RoleAdmin})
	if err != nil {
		log.Printf("[bootstrap] could not check for an existing admin: %v", err)
		return
	}
	if existing.Total > 0 {
		return
	}

	if !cfg.Bootstrap.Wanted() {
		log.Print("[bootstrap] no admin account exists; set BOOTSTRAP_ADMIN_EMAIL " +
			"and BOOTSTRAP_ADMIN_PASSWORD to create one on the next start")
		return
	}

	created, err := service.Create(ctx, &domain.UserCreateInput{
		Email:     cfg.Bootstrap.AdminEmail,
		Password:  cfg.Bootstrap.AdminPassword,
		FirstName: "Initial",
		LastName:  "Administrator",
		Role:      domain.RoleAdmin,
		Status:    domain.StatusActive,
	})
	if err != nil {
		// A conflict means somebody already holds the address -- as an ordinary
		// user, since the admin check above found none. That is a decision for a
		// person, not something to resolve by overwriting an account.
		if errors.Is(err, domain.ErrConflict) {
			log.Printf("[bootstrap] %s is already registered; promote that account manually",
				cfg.Bootstrap.AdminEmail)
			return
		}
		log.Printf("[bootstrap] could not create the first admin: %v", err)
		return
	}

	log.Printf("[bootstrap] created the first admin account %s (%s); "+
		"remove BOOTSTRAP_ADMIN_PASSWORD from the environment now",
		created.Email, created.ID)
}
