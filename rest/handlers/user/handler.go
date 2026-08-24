// Package user is the HTTP transport for the user resource.
package user

import (
	middlewares "go-commerce/rest/middlewares"
	usersvc "go-commerce/user"
)

type Handler struct {
	service     usersvc.Service
	middlewares *middlewares.Manager
}

func NewHandler(service usersvc.Service, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		service:     service,
		middlewares: moduleMiddlewares,
	}
}
