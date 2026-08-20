package user

import (
	db "go-commerce/database"
	middleware "go-commerce/rest/middlewares"
)

type UserStore interface {
	All() []db.User
	ByID(id int) (db.User, bool)
	Create(p db.User) db.User
	Update(id int, apply func(*db.User)) (db.User, bool)
	Delete(id int) bool
}

type Handler struct {
	userStore   UserStore
	middlewares *middleware.Manager
}

func NewHandler(userStore UserStore, middlewares *middleware.Manager) *Handler {
	if middlewares == nil {
		middlewares = middleware.NewManager()
	}

	return &Handler{
		userStore:   userStore,
		middlewares: middlewares,
	}
}
