package user

import (
	"net/http"

	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
	usersvc "go-commerce/user"
)

// createRequest is an admin-side user creation.
//
// Role and status are accepted here, unlike on public registration, because this
// route is behind an admin gate -- creating a colleague with the staff role is
// the point of it.
type createRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	FirstName   string  `json:"firstName"`
	LastName    string  `json:"lastName"`
	AvatarURL   *string `json:"avatarUrl"`
	PhoneNumber *string `json:"phoneNumber"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body createRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	created, err := h.service.Create(r.Context(), usersvc.CreateInput{
		Email:       body.Email,
		Password:    body.Password,
		FirstName:   body.FirstName,
		LastName:    body.LastName,
		AvatarURL:   body.AvatarURL,
		PhoneNumber: body.PhoneNumber,
		Role:        body.Role,
		Status:      body.Status,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := userLinks(created)
	w.Header().Set("Location", links["self"])
	response.Item(w, http.StatusCreated, created, links)
}
