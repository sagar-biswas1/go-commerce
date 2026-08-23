package cmd

import (
	"go-commerce/config"
	db "go-commerce/database"
	"go-commerce/repo"
	"go-commerce/rest"
	auth "go-commerce/rest/handlers/auth"
	product "go-commerce/rest/handlers/product"
	"go-commerce/rest/handlers/user"
	middlewares "go-commerce/rest/middlewares"
	"log"
)

// Serve is the composition root: the single place that picks concrete
// implementations and wires them together. Everything it builds is handed its
// dependencies, so no package below this one has to know what it is running on.
func Serve() {
	cfg := config.GetConfig()
	productRepo := repo.NewProductRepo()

	m := middlewares.NewMiddleWares(cfg)
	productPipeline := m.NewManager().Use(m.ProductLogger)
	productHandler := product.NewHandler(productRepo, productPipeline)

	userStore := db.NewUserStore()
	userPipeline := m.NewManager()
	userHandler := user.NewHandler(userStore, userPipeline)

	refreshTokenStore := db.NewRefreshTokenStore()
	authStore := auth.NewStore(userStore, refreshTokenStore)
	authPipeline := m.NewManager()
	authHandler := auth.NewHandler(cfg, authStore, authPipeline)

	// The global pipeline, applied to every request. The first middleware
	// registered is the outermost, so Logger times the whole chain.
	globalPipeline := m.NewManager().Use(m.Logger, m.CorsWithPreflight)

	server := rest.NewServer(globalPipeline, productHandler, userHandler, authHandler)

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
