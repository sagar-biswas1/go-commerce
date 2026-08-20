package user

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := h.userID(w, r)
	if !ok {
		return
	}

	if !h.userStore.Delete(id) {
		utils.SendError(w, "user not found", http.StatusNotFound)
		return
	}

	// 204 is the conventional reply for a delete with nothing left to return.
	w.WriteHeader(http.StatusNoContent)
}
