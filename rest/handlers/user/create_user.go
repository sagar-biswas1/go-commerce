package user

import (
	"encoding/json"
	db "go-commerce/database"
	"go-commerce/utils"
	"net/http"
	"strconv"
)

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var newUser db.User

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&newUser); err != nil {
		utils.SendError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	v := NewUserValidator()

	if !v.ValidateFull(newUser) {

		utils.SendError(w, utils.StringifyErrors(v.Errors), http.StatusBadRequest)
		return
	}

	created := h.userStore.Create(newUser)
	w.Header().Set("Location", "/users/"+strconv.Itoa(created.ID))
	utils.SendData(w, created, http.StatusCreated)
}
