package helpers

import (
	"errors"
	"go-commerce/config"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func fmtInt(i int) string { return strconv.Itoa(i) }
func atoi(s string) int   { v, _ := strconv.Atoi(s); return v }

var (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour

	ErrInvalidToken = errors.New("invalid or expired token")
)

func accessTokenSecret() []byte {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config failed to load: %v", err)
	}
	return []byte(strings.TrimSpace(cfg.JWTAccessSecret))
}

func refreshTokenSecret() []byte {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config failed to load: %v", err)
	}
	return []byte(strings.TrimSpace(cfg.JWTRefreshSecret))
}

type Claims struct {
	UserID int    `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a short-lived JWT for authenticating requests.
func GenerateAccessToken(userID int, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(accessTokenSecret())
}

// GenerateRefreshToken creates a longer-lived JWT used for session renewal.
func GenerateRefreshToken(userID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(RefreshTokenTTL)

	claims := jwt.RegisteredClaims{
		Subject:   itoa(userID),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(refreshTokenSecret())
	return signed, expiresAt, err
}

// ParseAccessToken validates an access token and returns its claims.
func ParseAccessToken(tokenStr string) (*Claims, error) {

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return accessTokenSecret(), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func itoa(i int) string {
	return fmtInt(i)
}

// ParseRefreshToken validates a refresh token and returns the user ID.
func ParseRefreshToken(tokenStr string) (int, error) {

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return refreshTokenSecret(), nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}
	return atoi(claims.Subject), nil
}
