package middlewares

import "go-commerce/config"

type Middlewares struct {
	cfg *config.Config
}

func NewMiddleWares(cfg *config.Config) *Middlewares {
	return &Middlewares{
		cfg: cfg,
	}
}
