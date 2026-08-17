package productHandlers

import (
	"go-commerce/utils"
	"net/http"
	"strconv"
	"strings"
)

// productID reads the {id} path segment, replying 400 when it is not usable.
func productID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err:= strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
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
