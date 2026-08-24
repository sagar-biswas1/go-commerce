// Package cmd is the composition root: the single place that picks concrete
// implementations and wires them together.
//
// Everything it builds is handed its dependencies, so no package below this one
// has to know what it is running on. That is what the whole layout is for -- the
// dependency arrows all point inward at domain, and this file is the only place
// where the outward ones are drawn.
package cmd

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"go-commerce/auth"
	"go-commerce/config"
	"go-commerce/infra/db"
	"go-commerce/infra/security"
	"go-commerce/migration"
	"go-commerce/product"
	"go-commerce/repo/authrepo"
	"go-commerce/repo/productrepo"
	"go-commerce/repo/userrepo"
	"go-commerce/rest"
	authhandler "go-commerce/rest/handlers/auth"
	producthandler "go-commerce/rest/handlers/product"
	roothandler "go-commerce/rest/handlers/root"
	userhandler "go-commerce/rest/handlers/user"
	middlewares "go-commerce/rest/middlewares"
	"go-commerce/user"
)

// Serve builds the application and runs it until it is told to stop.
func Serve() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("configuration failed to load: %v", err)
	}

	// One context for the whole process, cancelled by an interrupt or a SIGTERM.
	// It reaches the HTTP server and the background janitor, so a single signal
	// stops everything in an orderly way rather than killing the process
	// mid-request.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Infrastructure ---------------------------------------------------

	dbCon, err := db.GetConnection(cfg.PG)
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}
	defer dbCon.Close()

	// The schema is applied before anything is served, so the process either
	// starts against a database it agrees with or does not start at all.
	//
	// This is the same runner the migrate CLI drives, holding the same advisory
	// lock -- so several instances starting together do not race, and one of
	// them migrating does not make the others fail.
	if err := db.Migrate(ctx, dbCon, migration.FS); err != nil {
		log.Fatalf("could not migrate the database: %v", err)
	}

	hasher := security.GetPasswordHasher()
	issuer := security.GetJWTIssuer(cfg.JWT)

	// --- Repositories: the adapters that satisfy the service ports ---------
	//
	// One package per aggregate, each importing only its own. See ports.go for
	// the two contracts that deliberately span two aggregates.

	productRepo := productrepo.NewProductRepo(dbCon)
	userRepo := userrepo.NewUserRepo(dbCon)
	tokenRepo := authrepo.NewRefreshTokenRepo(dbCon)

	// --- Application services ---------------------------------------------

	productService := product.GetService(productRepo)
	userService := user.GetService(userRepo, hasher, tokenRepo)
	authService := auth.GetService(userRepo, tokenRepo, issuer, hasher)

	// A database with no admin cannot be administered, so make the first one if
	// the environment asks for it. This runs before anything is served, so the
	// account exists by the time the port is open.
	bootstrapAdmin(ctx, cfg, userService, userRepo)

	// Refresh tokens accumulate with every login and every rotation, so a
	// janitor sweeps the dead ones. It shares the process context, so the signal
	// that stops the server stops it too.
	auth.GetJanitor(tokenRepo).Start(ctx)

	// --- Transport --------------------------------------------------------

	// The middleware layer is given the token validator rather than building
	// one, which is what keeps it independent of how tokens are signed.
	m := middlewares.NewMiddleWares(cfg, issuer)

	productHandler := producthandler.NewHandler(productService, m.NewManager().Use(m.ProductLogger))
	userHandler := userhandler.NewHandler(userService, m.NewManager())
	// A Secure cookie is not sent over plain HTTP at all, so hardcoding it would
	// either break local development or ship a session cookie that travels in
	// the clear. Tying it to the environment means production gets the safe
	// answer without anyone having to remember to ask for it.
	authHandler := authhandler.NewHandler(authService, m.NewManager(), !cfg.IsDevelopment())
	rootHandler := roothandler.NewHandler(cfg, dbCon, m.NewManager())

	// The global pipeline, applied to every request. The first middleware
	// registered is the outermost, so Recover covers everything inside it and
	// Logger times the whole chain.
	globalPipeline := m.NewManager().Use(m.Recover, m.Logger, m.CorsWithPreflight)

	server := rest.NewServer(cfg, globalPipeline,
		rootHandler, productHandler, userHandler, authHandler)

	if err := server.Start(ctx); err != nil {
		log.Fatalf("server stopped with an error: %v", err)
	}
}
