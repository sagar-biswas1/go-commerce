package auth

import (
	"encoding/json"
	"log"
	"net/http"

	db "go-commerce/database"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	log.Printf("Payload: %+v\n", req)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	user, ok := h.authStore.ByEmail(req.Email)

	if !ok {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		log.Printf("Payload: %+v\n", err)
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	accessToken, err := h.jwtHelper.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	refreshTokenStr, expiresAt, err := h.jwtHelper.GenerateRefreshToken(user.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.authStore.CreateRefreshToken(db.RefreshToken{
		UserID:    user.ID,
		Token:     refreshTokenStr,
		ExpiresAt: expiresAt,
	})

	setRefreshTokenCookie(w, refreshTokenStr, expiresAt)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"access_token": accessToken,
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	}); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
