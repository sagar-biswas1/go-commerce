package user

import (
	"encoding/json"
	db "go-commerce/database"
	"go-commerce/utils"
	"net/http"
	"time"
)

// UserPatch mirrors db.User for PATCH bodies. Pointers let us tell
// "field omitted" apart from "field set to zero".
type UserPatch struct {
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
	id, ok := h.userID(w, r)
	if !ok {
		return
	}

	var updates UserPatch
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.SendError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	v := NewUserValidator()
	if !v.ValidatePartial(updates) {
		utils.SendError(w, utils.StringifyErrors(v.Errors), http.StatusBadRequest)
		return
	}

	updated, found := h.userStore.Update(id, func(u *db.User) {
		if updates.Email != nil {
			u.Email = *updates.Email
		}
		if updates.FirstName != nil {
			u.FirstName = *updates.FirstName
		}
		if updates.LastName != nil {
			u.LastName = *updates.LastName
		}
		if updates.AvatarURL != nil {
			u.AvatarURL = updates.AvatarURL
		}
		if updates.PhoneNumber != nil {
			u.PhoneNumber = updates.PhoneNumber
		}
		if updates.Role != nil {
			u.Role = *updates.Role
		}
		if updates.Status != nil {
			u.Status = *updates.Status
		}
		if updates.IsEmailVerified != nil {
			u.IsEmailVerified = *updates.IsEmailVerified
		}
		u.UpdatedAt = time.Now().UTC()
	})

	if !found {
		utils.SendError(w, "User not found", http.StatusNotFound)
		return
	}

	utils.SendData(w, updated, http.StatusOK)
}
