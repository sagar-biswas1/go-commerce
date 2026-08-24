package user

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
	usersvc "go-commerce/user"
)

// patchRequest mirrors the updatable fields. Pointers tell an omitted field
// apart from one set to its zero value.
//
// Role, status and isEmailVerified are accepted from any caller and then refused
// by the service unless that caller is an admin. Rejecting them here instead
// would put an authorization rule in the transport layer, where the next
// endpoint that touches a user would have to reimplement it.
type patchRequest struct {
	Email           *string `json:"email"`
	FirstName       *string `json:"firstName"`
	LastName        *string `json:"lastName"`
	AvatarURL       *string `json:"avatarUrl"`
	PhoneNumber     *string `json:"phoneNumber"`
	Role            *string `json:"role"`
	Status          *string `json:"status"`
	IsEmailVerified *bool   `json:"isEmailVerified"`
}

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	id, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	var body patchRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	updated, err := h.service.Update(r.Context(), middlewares.MustIdentity(r.Context()), id, usersvc.Patch{
		Email:           body.Email,
		FirstName:       body.FirstName,
		LastName:        body.LastName,
		AvatarURL:       body.AvatarURL,
		PhoneNumber:     body.PhoneNumber,
		Role:            body.Role,
		Status:          body.Status,
		IsEmailVerified: body.IsEmailVerified,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	response.Item(w, http.StatusOK, updated, userLinks(updated))
}
