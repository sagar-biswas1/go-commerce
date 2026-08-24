package user

import (
	"go-commerce/domain"
	"go-commerce/rest/response"
)

const basePath = "/users"

func userLinks(u domain.User) response.Links {
	self := response.Path(basePath, u.ID.String())

	return response.Links{
		"self":           self,
		"update":         self,
		"delete":         self,
		"changePassword": response.Path(basePath, u.ID.String(), "password"),
		"collection":     basePath,
	}
}
