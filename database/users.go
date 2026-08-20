package database

import (
	"sync"
	"time"
)

type User struct {
	ID              int        `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	Password        string     `json:"-" db:"password_hash"`
	FirstName       string     `json:"firstName" db:"first_name"`
	LastName        string     `json:"lastName" db:"last_name"`
	AvatarURL       *string    `json:"avatarUrl,omitempty" db:"avatar_url"`
	PhoneNumber     *string    `json:"phoneNumber,omitempty" db:"phone_number"`
	Role            string     `json:"role" db:"role"`
	Status          string     `json:"status" db:"status"`
	IsEmailVerified bool       `json:"isEmailVerified" db:"is_email_verified"`
	LastLoginAt     *time.Time `json:"lastLoginAt,omitempty" db:"last_login_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" db:"updated_at"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty" db:"deleted_at"`
}

type UserStore struct {
	mu     sync.RWMutex
	nextID int
	users  []User
}

func NewUserStore() *UserStore {
	now := time.Now().UTC()

	return &UserStore{
		nextID: 2, // next available id after the single seed user (ID: 1)
		users: []User{
			{
				ID:              1,
				Email:           "sagar@example.com",
				Password:        "$2a$10$EixZaYVK1fsbw1ZfbX3OXePaWxn96p36WQoeg6Lruj3vjPGga31lW", // bcrypt for "123456"
				FirstName:       "Sagar",
				LastName:        "Biswas",
				AvatarURL:       stringPtr("https://cdn.example.com/avatars/sagar.jpg"),
				PhoneNumber:     stringPtr("+8801700000000"),
				Role:            "admin",
				Status:          "active",
				IsEmailVerified: true,
				LastLoginAt:     &now,
				CreatedAt:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				UpdatedAt:       now,
				DeletedAt:       nil,
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}

func (u *UserStore) All() []User {
	u.mu.RLock()
	defer u.mu.RUnlock()

	out := make([]User, len(u.users))
	copy(out, u.users)
	return out
}

func (u *UserStore) ByID(id int) (User, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, p := range u.users {
		if p.ID == id {
			return p, true
		}
	}
	return User{}, false
}

func (u *UserStore) Create(p User) User {
	u.mu.Lock()
	defer u.mu.Unlock()

	p.ID = u.nextID
	u.nextID++
	u.users = append(u.users, p)
	return p
}

func (u *UserStore) Update(id int, apply func(*User)) (User, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for i := range u.users {
		if u.users[i].ID == id {
			apply(&u.users[i])
			u.users[i].ID = id
			return u.users[i], true
		}
	}
	return User{}, false
}

func (u *UserStore) Delete(id int) bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	for i, p := range u.users {
		if p.ID == id {
			u.users = append(u.users[:i], u.users[i+1:]...)
			return true
		}
	}
	return false
}
