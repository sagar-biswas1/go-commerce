package rest

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"go-commerce/config"
	middlewares "go-commerce/rest/middlewares"
)

// Server owns the HTTP transport: it runs a mux behind a middleware pipeline.
//
// It deliberately knows nothing about which resources it serves or which
// middleware are in the pipeline -- both arrive through NewServer, which is why
// this package imports no handler package at all.
type Server struct {
	config            *config.Config
	globalMiddlewares *middlewares.Manager
	registrars        []RouteRegistrar
	mux               *http.ServeMux
}

// Timeouts. A server with none will happily hold a connection open forever,
// which is all it takes for a slow client -- or a few thousand of them -- to
// exhaust the process.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second

	// shutdownGrace is how long in-flight requests get to finish once a stop is
	// requested.
	shutdownGrace = 15 * time.Second
)

// NewServer takes its dependencies instead of constructing them: the config to
// listen with, the global pipeline to wrap every request in, and the resources
// to mount. Adding a resource means passing one more registrar here, with no
// edit to this file.
func NewServer(cfg *config.Config, globalMiddlewares *middlewares.Manager, registrars ...RouteRegistrar) *Server {
	server := Server{
		config:            cfg,
		globalMiddlewares: globalMiddlewares,
		registrars:        registrars,
		mux:               http.NewServeMux(),
	}
	server.registerRoutes()

	return &server
}

// Start listens until ctx is cancelled, then drains.
//
// The two-phase stop is what makes a deploy or a Ctrl-C not drop requests:
// Shutdown stops accepting new connections and waits out the ones in flight, so
// a client mid-request gets its answer instead of a reset. Past the grace period
// the remainder are closed, because a hung handler must not stop the process
// from ever exiting.
func (s *Server) Start(ctx context.Context) error {
	if s.config == nil {
		return errors.New("rest: server started without a configuration")
	}
	if s.globalMiddlewares == nil {
		return errors.New("rest: server started without a global middleware pipeline")
	}

	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(s.config.HttpPort),
		Handler: s.globalMiddlewares.ThenHandler(s.mux),

		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// ListenAndServe blocks, so it runs in its own goroutine and reports back
	// through this channel. Buffered, so the goroutine can finish even if
	// nobody is left to read from it.
	serveErr := make(chan error, 1)

	go func() {
		log.Println(s.config.ServiceName, s.config.Version, "listening on", httpServer.Addr)
		serveErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// ErrServerClosed is what a deliberate Shutdown produces, so it is not
		// a failure -- but reaching it here means something else closed the
		// server, which is worth reporting.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		log.Println("shutting down; waiting up to", shutdownGrace, "for in-flight requests")

		// A context of its own: the one that just fired is already cancelled, so
		// passing it to Shutdown would close every connection immediately.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return errors.Join(errors.New("rest: shutdown did not complete"), err)
		}

		log.Println("shutdown complete")
		return nil
	}
}
