package cmd

import (
	"go-commerce/config"
	db "go-commerce/database"
	"go-commerce/rest"
	auth "go-commerce/rest/handlers/auth"
	product "go-commerce/rest/handlers/product"
	"go-commerce/rest/handlers/user"
	middleware "go-commerce/rest/middlewares"
	"log"
)

// Serve is the composition root: the single place that picks concrete
// implementations and wires them together. Everything it builds is handed its
// dependencies, so no package below this one has to know what it is running on.
func Serve(cfg *config.Config) {
	// Data layer. Swapping in a SQL-backed store is a change to this line only.
	productStore := db.NewProductStore()
	userStore := db.NewUserStore()
	// The product module's own pipeline, applied to product routes only.
	productPipeline := middleware.NewManager().
		Use(middleware.ProductLogger)

	// The user module has no module-wide middleware yet, so it gets an empty
	// pipeline. Its write routes attach their own -- see user.RegisterRoutes.
	userPipeline := middleware.NewManager()

	productHandler := product.NewHandler(productStore, productPipeline)

	userHandler := user.NewHandler(userStore, userPipeline)

	refreshTokenStore := db.NewRefreshTokenStore()

	authStore := auth.NewStore(userStore, refreshTokenStore)

	authHandler := auth.NewHandler(authStore, middleware.NewManager())

	// The global pipeline, applied to every request. The first middleware
	// registered is the outermost, so Logger times the whole chain.
	globalPipeline := middleware.NewManager().
		Use(middleware.Logger).
		Use(middleware.CorsWithPreflight)

	server := rest.NewServer(cfg, globalPipeline, productHandler, userHandler, authHandler)

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
