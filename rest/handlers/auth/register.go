package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	db "go-commerce/database"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Role      string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if _, ok := h.authStore.ByEmail(req.Email); ok {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user := db.User{
		Email:           req.Email,
		Password:        string(hashedPassword),
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Role:            "user",
		Status:          "active",
		IsEmailVerified: false,
	}
	if req.Role != "" {
		user.Role = req.Role
	}

	created := h.authStore.CreateUser(user)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":    created.ID,
		"email": created.Email,
		"role":  created.Role,
	}); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
