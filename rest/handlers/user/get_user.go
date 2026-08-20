package user

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) GetUserById(w http.ResponseWriter, r *http.Request) {
	id, ok := h.userID(w, r)
	if !ok {
		return
	}

	found, exists := h.userStore.ByID(id)
	if !exists {
		utils.SendError(w, "user not found", http.StatusNotFound)
		return
	}

	utils.SendData(w, found, http.StatusOK)
}
