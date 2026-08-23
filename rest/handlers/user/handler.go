package user

import (
	db "go-commerce/database"
	middlewares "go-commerce/rest/middlewares"
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
	middlewares *middlewares.Manager
}

func NewHandler(userStore UserStore, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		userStore:   userStore,
		middlewares: moduleMiddlewares,
	}
}
