package rest

import (
	productHandlers "go-commerce/rest/product/handlers"
)

// registerRoutes mounts every resource on the server mux.
func (s *Server) registerRoutes() {
	productHandlers.RegisterRoutes(s.mux)
}
