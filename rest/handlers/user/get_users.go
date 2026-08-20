package user

import (
	"go-commerce/utils"
	"net/http"
)

func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	utils.SendData(w, h.userStore.All(), http.StatusOK)
}
