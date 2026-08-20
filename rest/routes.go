package rest

import "net/http"

// RouteRegistrar is anything that can mount itself on a mux. Each resource
// package satisfies it with its own RegisterRoutes method, so the server
// depends on this one-method interface rather than on the resources.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// registerRoutes lets every injected resource mount its own route table.
func (s *Server) registerRoutes() {
	for _, registrar := range s.registrars {
		registrar.RegisterRoutes(s.mux)
	}
}
