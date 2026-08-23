package helpers

import (
	"errors"
	"go-commerce/config"
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

type JWTHelper struct {
	cfg *config.Config
}

func NewJWTHelper(cfg *config.Config) *JWTHelper {
	return &JWTHelper{
		cfg: cfg,
	}
}

func (c *JWTHelper) accessTokenSecret() []byte {
	return []byte(strings.TrimSpace(c.cfg.JWTAccessSecret))
}

func (c *JWTHelper) refreshTokenSecret() []byte {

	return []byte(strings.TrimSpace(c.cfg.JWTRefreshSecret))
}

type Claims struct {
	UserID int    `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a short-lived JWT for authenticating requests.
func (c *JWTHelper) GenerateAccessToken(userID int, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(c.accessTokenSecret())
}

// GenerateRefreshToken creates a longer-lived JWT used for session renewal.
func (c *JWTHelper) GenerateRefreshToken(userID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(RefreshTokenTTL)

	claims := jwt.RegisteredClaims{
		Subject:   itoa(userID),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(c.refreshTokenSecret())
	return signed, expiresAt, err
}

// ParseAccessToken validates an access token and returns its claims.
func (c *JWTHelper) ParseAccessToken(tokenStr string) (*Claims, error) {

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return c.accessTokenSecret(), nil
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
func (c *JWTHelper) ParseRefreshToken(tokenStr string) (int, error) {

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return c.refreshTokenSecret(), nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}
	return atoi(claims.Subject), nil
}
