package auth

import (
	"go-commerce/config"
	db "go-commerce/database"
	"go-commerce/rest/helpers"
	middlewares "go-commerce/rest/middlewares"
)

type AuthStore interface {
	ByEmail(email string) (db.User, bool)
	CreateUser(db.User) db.User
	CreateRefreshToken(db.RefreshToken) db.RefreshToken
	GetRefreshToken(string) (db.RefreshToken, bool)
	RevokeRefreshToken(string) error
}

type Store struct {
	userStore    *db.UserStore
	refreshStore *db.RefreshTokenStore
}

func NewStore(userStore *db.UserStore, refreshStore *db.RefreshTokenStore) *Store {
	return &Store{
		userStore:    userStore,
		refreshStore: refreshStore,
	}
}

func (s *Store) ByEmail(email string) (db.User, bool) {
	if s == nil || s.userStore == nil {
		return db.User{}, false
	}
	return s.userStore.ByEmail(email)
}

func (s *Store) CreateUser(user db.User) db.User {
	if s == nil || s.userStore == nil {
		return user
	}
	return s.userStore.Create(user)
}

func (s *Store) CreateRefreshToken(token db.RefreshToken) db.RefreshToken {
	if s == nil || s.refreshStore == nil {
		return token
	}
	return s.refreshStore.CreateRefreshToken(token)
}

func (s *Store) GetRefreshToken(token string) (db.RefreshToken, bool) {
	if s == nil || s.refreshStore == nil {
		return db.RefreshToken{}, false
	}
	return s.refreshStore.GetRefreshToken(token)
}

func (s *Store) RevokeRefreshToken(token string) error {
	if s == nil || s.refreshStore == nil {
		return db.ErrRefreshTokenNotFound
	}
	return s.refreshStore.RevokeRefreshToken(token)
}

type Handler struct {
	authStore   AuthStore
	middlewares *middlewares.Manager
	jwtHelper   *helpers.JWTHelper
}

func NewHandler(cfg *config.Config, authStore AuthStore, moduleMiddlewares *middlewares.Manager) *Handler {
	return &Handler{
		authStore:   authStore,
		middlewares: moduleMiddlewares,
		jwtHelper:   helpers.NewJWTHelper(cfg),
	}
}
