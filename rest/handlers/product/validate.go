package product

import (
	"go-commerce/rest/helpers"
	"go-commerce/utils"
	"net/http"
	"strings"
)

// productID reads the {id} path segment and replies 400 itself when it is not
// usable, so each handler can stay a straight line.
func (h *Handler) productID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := helpers.PathInt(r, "id")
	if err != nil {
		utils.SendError(w, "Invalid product ID", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// validateTitle and validatePrice describe what a product body must contain.
func validateTitle(title string) (string, bool) {
	if strings.TrimSpace(title) == "" {
		return "title is required", false
	}
	return "", true
}

func validatePrice(price float64) (string, bool) {
	if price < 0 {
		return "price must not be negative", false
	}
	return "", true
}
