package rest

import (
	"go-commerce/config"
	middleware "go-commerce/rest/middlewares"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Server owns the HTTP transport layer: the route table and the middleware pipeline.
type Server struct {
	config config.Config
	mux    *http.ServeMux
}

func NewServer(cfg config.Config) *Server {
	server := Server{
		config: cfg,
		mux:    http.NewServeMux(),
	}
	server.registerRoutes()

	return &server
}

func (s *Server) Start() error {
	pipeline := middleware.NewManager().
		Use(middleware.Logger).
		Use(middleware.CorsWithPreflight)

	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(s.config.HttpPort),
		Handler: pipeline.ThenHandler(s.mux),

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println(s.config.ServiceName, s.config.Version, "listening on", httpServer.Addr)

	return httpServer.ListenAndServe()
}
