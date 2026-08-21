package database

import (
	"errors"
	"sync"
	"time"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type RefreshToken struct {
	ID        int        `json:"id" db:"id"`
	UserID    int        `json:"userId" db:"user_id"`
	Token     string     `json:"token" db:"token"`
	ExpiresAt time.Time  `json:"expiresAt" db:"expires_at"`
	RevokedAt *time.Time `json:"revokedAt,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
}

type RefreshTokenStore struct {
	mu     sync.RWMutex
	nextID int
	tokens []RefreshToken
}

func NewRefreshTokenStore() *RefreshTokenStore {
	return &RefreshTokenStore{
		nextID: 1,
		tokens: []RefreshToken{},
	}
}

func (s *RefreshTokenStore) CreateRefreshToken(t RefreshToken) RefreshToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.ID = s.nextID
	s.nextID++
	t.CreatedAt = time.Now().UTC()

	s.tokens = append(s.tokens, t)
	return t
}

func (s *RefreshTokenStore) GetRefreshToken(token string) (RefreshToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.tokens {
		if t.Token == token {
			return t, true
		}
	}
	return RefreshToken{}, false
}

func (s *RefreshTokenStore) RevokeRefreshToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tokens {
		if s.tokens[i].Token == token {
			now := time.Now().UTC()
			s.tokens[i].RevokedAt = &now
			return nil
		}
	}
	return ErrRefreshTokenNotFound
}
