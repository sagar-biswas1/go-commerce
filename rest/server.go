package rest

import (
	"errors"
	"go-commerce/config"
	middlewares "go-commerce/rest/middlewares"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Server owns the HTTP transport layer: it runs a mux behind a middleware
// pipeline. It deliberately knows nothing about which resources it serves or
// which middleware are in the pipeline -- both arrive through NewServer, which
// is why this package imports no handler package at all.
type Server struct {
	config            *config.Config
	globalMiddlewares *middlewares.Manager
	registrars        []RouteRegistrar
	mux               *http.ServeMux
}

// NewServer takes its dependencies instead of constructing them: the config to
// listen with, the global pipeline to wrap every request in, and the resources
// to mount. Adding a resource means passing one more registrar here, with no
// edit to this file.
func NewServer(globalMiddlewares *middlewares.Manager, registrars ...RouteRegistrar) *Server {
	cfg := config.GetConfig()
	server := Server{
		config:            cfg,
		globalMiddlewares: globalMiddlewares,
		registrars:        registrars,
		mux:               http.NewServeMux(),
	}
	server.registerRoutes()

	return &server
}

func (s *Server) Start() error {
	// The config arrives by injection, so a caller can forget it. Say so
	// plainly instead of panicking on the first field access.
	if s.config == nil {
		return errors.New("rest: server started without a configuration")
	}
	if s.globalMiddlewares == nil {
		return errors.New("rest: server started without a global middleware pipeline")
	}
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(s.config.HttpPort),
		Handler: s.globalMiddlewares.ThenHandler(s.mux),

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println(s.config.ServiceName, s.config.Version, "listening on", httpServer.Addr)

	return httpServer.ListenAndServe()
}
