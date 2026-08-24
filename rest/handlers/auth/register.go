package auth

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/response"
)

// registerRequest is a public registration.
//
// There is no role field, and that is the point. The old handler accepted one and
// honoured it, so anyone could register as an admin by adding a line to the body.
// A new account is always an ordinary user; promoting one is an admin action on
// /users/{id}.
type registerRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body registerRequest
	if err := helpers.DecodeJSON(w, r, &body); err != nil {
		response.Fail(w, r, err)
		return
	}

	created, err := h.service.Register(r.Context(), &domain.RegisterInput{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
		Password:  body.Password,
	})
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	// Registration does not sign the user in: no tokens are minted here, so a
	// client has to log in with the credentials it just chose and prove they
	// work.
	response.Item(w, http.StatusCreated, created, response.Links{
		"self":  response.Path("/users", created.ID.String()),
		"login": basePath + "/login",
	})
}
