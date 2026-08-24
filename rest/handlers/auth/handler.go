// Package auth is the HTTP transport for authentication.
package auth

import (
	middlewares "go-commerce/rest/middlewares"
)

type Handler struct {
	service     Service
	middlewares *middlewares.Manager
	// secureCookies controls the Secure attribute on the refresh cookie. It is
	// injected rather than hardcoded because a cookie marked Secure is not sent
	// over plain HTTP at all, so hardcoding it either breaks local development
	// or ships a cookie that travels in the clear.
	secureCookies bool
}

func NewHandler(service Service, moduleMiddlewares *middlewares.Manager, secureCookies bool) *Handler {
	return &Handler{
		service:       service,
		middlewares:   moduleMiddlewares,
		secureCookies: secureCookies,
	}
}
