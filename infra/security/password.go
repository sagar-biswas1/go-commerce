// Package security holds the cryptographic adapters: the concrete answers to
// the hashing and signing ports the service packages declare.
package security

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher satisfies the PasswordHasher port that the user and auth
// services declare.
//
// It exists as a type rather than as two direct bcrypt calls so the cost can be
// turned down in tests: bcrypt at production cost makes a suite that creates
// users slow enough that people stop running it.
type BcryptHasher struct {
	cost int
}

var (
	once     sync.Once
	instance *BcryptHasher
)

// GetPasswordHasher returns the process-wide hasher.
func GetPasswordHasher() *BcryptHasher {
	once.Do(func() {
		instance = NewBcryptHasher(bcrypt.DefaultCost)
	})
	return instance
}

// NewBcryptHasher builds a hasher at the given cost, clamping anything bcrypt
// would refuse. A cost below the library's minimum would otherwise fail at the
// first registration rather than here.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plaintext string) (string, error) {
	// bcrypt ignores everything past 72 bytes, so a longer password would be
	// stored and then only partly checked. The domain rejects those, and this is
	// the backstop for any caller that skipped it.
	if len(plaintext) > 72 {
		return "", errors.New("password is too long to hash")
	}

	digest, err := bcrypt.GenerateFromPassword([]byte(plaintext), h.cost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(digest), nil
}

// Compare reports whether plaintext produced hashed. The error is returned
// as-is and never inspected by callers, which treat any failure as "wrong
// password" -- distinguishing a malformed digest from a mismatch would tell a
// caller something about the stored value.
func (h *BcryptHasher) Compare(hashed, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext))
}
