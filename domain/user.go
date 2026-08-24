package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is a person who can sign in.
//
// Password holds a bcrypt digest and never leaves the process: the `json:"-"`
// tag is the only thing between that digest and every handler that marshals a
// user, so it stays.
type User struct {
	ID              uuid.UUID  `json:"id" db:"id"`
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

// The roles and statuses a user may hold. They live beside the rules that read
// them, and the schema's CHECK constraints mirror this list, so storage and
// validation cannot drift apart.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	RoleStaff = "staff"

	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
	StatusPending   = "pending"
)

var (
	validRoles = map[string]bool{
		RoleAdmin: true,
		RoleUser:  true,
		RoleStaff: true,
	}
	validStatuses = map[string]bool{
		StatusActive:    true,
		StatusInactive:  true,
		StatusSuspended: true,
		StatusPending:   true,
	}
)

var (
	ErrUserNotFound = wrap("user not found", ErrNotFound)
	ErrEmailTaken   = wrap("email already registered", ErrConflict)
	// ErrInvalidCredentials is deliberately one error for both "no such email"
	// and "wrong password". Two distinct messages would let anyone test which
	// addresses have accounts.
	ErrInvalidCredentials = wrap("invalid email or password", ErrUnauthorized)
	ErrUserNotActive      = wrap("account is not active", ErrForbidden)
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{7,14}$`) // E.164-ish, 8-15 digits
)

// Field limits.
const (
	MaxEmailLen    = 254
	MaxNameLen     = 100
	MinPasswordLen = 8
	// bcrypt silently ignores everything past 72 bytes, so a longer password
	// would be accepted and then only partly checked.
	MaxPasswordLen = 72
)

// NormalizeEmail is the one definition of how an address is compared and
// stored. The schema's unique index is on lower(email) to match it, so
// "Sagar@example.com" cannot become a second account for someone who has one.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Normalize trims the fields a client typed and canonicalizes the email.
func (u *User) Normalize() {
	u.Email = NormalizeEmail(u.Email)
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)
	if u.Role == "" {
		u.Role = RoleUser
	}
	if u.Status == "" {
		u.Status = StatusActive
	}
}

// CanAuthenticate is the rule that a session depends on. It is a method on the
// entity rather than a check in the login handler, so the refresh path and any
// future path enforce exactly the same thing.
func (u User) CanAuthenticate() bool {
	return u.DeletedAt == nil && u.Status == StatusActive
}

// IsAdmin reports whether this user may act on other people's resources.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// Validate checks a whole user, as on registration or an admin-side create.
// The plaintext password is passed separately because the entity only ever
// holds the digest.
func (u User) Validate(plaintextPassword string) error {
	v := NewValidationError()
	u.validateProfile(v)

	if msg, ok := ValidatePassword(plaintextPassword); !ok {
		v.Add("password", msg)
	}

	return v.OrNil()
}

// ValidateProfile checks everything but the password, for updates that do not
// touch it.
func (u User) ValidateProfile() error {
	v := NewValidationError()
	u.validateProfile(v)
	return v.OrNil()
}

func (u User) validateProfile(v *ValidationError) {
	if msg, ok := ValidateEmail(u.Email); !ok {
		v.Add("email", msg)
	}
	if msg, ok := ValidateName("firstName", u.FirstName); !ok {
		v.Add("firstName", msg)
	}
	if msg, ok := ValidateName("lastName", u.LastName); !ok {
		v.Add("lastName", msg)
	}
	if msg, ok := ValidatePhone(u.PhoneNumber); !ok {
		v.Add("phoneNumber", msg)
	}
	if msg, ok := ValidateRole(u.Role); !ok {
		v.Add("role", msg)
	}
	if msg, ok := ValidateStatus(u.Status); !ok {
		v.Add("status", msg)
	}
}

// The individual rules are exported so a partial update can run just the ones
// whose fields are present, without a second copy of each rule.
func ValidateEmail(email string) (string, bool) {
	email = strings.TrimSpace(email)
	switch {
	case email == "":
		return "email is required", false
	case len(email) > MaxEmailLen:
		return fmt.Sprintf("email must be %d characters or fewer", MaxEmailLen), false
	case !emailRegex.MatchString(email):
		return "email is not a valid email address", false
	}
	return "", true
}

func ValidatePassword(password string) (string, bool) {
	switch {
	case len(password) < MinPasswordLen:
		return fmt.Sprintf("password must be at least %d characters", MinPasswordLen), false
	case len(password) > MaxPasswordLen:
		return fmt.Sprintf("password must be %d characters or fewer", MaxPasswordLen), false
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case 'A' <= r && r <= 'Z':
			hasUpper = true
		case 'a' <= r && r <= 'z':
			hasLower = true
		case '0' <= r && r <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return "password must contain uppercase, lowercase, and a digit", false
	}
	return "", true
}

func ValidateName(field, name string) (string, bool) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return field + " is required", false
	case len(name) > MaxNameLen:
		return fmt.Sprintf("%s must be %d characters or fewer", field, MaxNameLen), false
	}
	return "", true
}

// ValidatePhone accepts a nil or blank number: the field is optional.
func ValidatePhone(phone *string) (string, bool) {
	if phone == nil {
		return "", true
	}
	trimmed := strings.TrimSpace(*phone)
	if trimmed == "" {
		return "", true
	}
	if !phoneRegex.MatchString(trimmed) {
		return "phoneNumber is not a valid phone number", false
	}
	return "", true
}

func ValidateRole(role string) (string, bool) {
	if role == "" {
		return "role is required", false
	}
	if !validRoles[role] {
		return "role must be one of: admin, staff, user", false
	}
	return "", true
}

func ValidateStatus(status string) (string, bool) {
	if status == "" {
		return "status is required", false
	}
	if !validStatuses[status] {
		return "status must be one of: active, inactive, pending, suspended", false
	}
	return "", true
}

// UserFilter narrows a listing.
type UserFilter struct {
	Search string // matches email, first name or last name
	Role   string
	Status string
	Sort   Sort
}

func (f UserFilter) Validate() error {
	v := NewValidationError()

	if f.Role != "" && !validRoles[f.Role] {
		v.Add("role", "role must be one of: admin, staff, user")
	}
	if f.Status != "" && !validStatuses[f.Status] {
		v.Add("status", "status must be one of: active, inactive, pending, suspended")
	}

	return v.OrNil()
}

// UserCreateInput is an admin-side user creation.
//
// The password arrives in plaintext and is hashed before it reaches the entity,
// which is why this is a separate type from User: User holds a digest and has
// nowhere to put a plaintext password, so no caller can hand one straight
// through to storage.
//
// It lives here for the same reason UserPatch does -- both the transport that
// builds one and the service that consumes one have to name it.
type UserCreateInput struct {
	Email       string
	Password    string
	FirstName   string
	LastName    string
	AvatarURL   *string
	PhoneNumber *string
	Role        string
	Status      string
}

// UserPatch is a partial change to a user. A nil field means "leave this alone",
// which is what tells an omitted field apart from one set to its zero value.
type UserPatch struct {
	Email           *string
	FirstName       *string
	LastName        *string
	AvatarURL       *string
	PhoneNumber     *string
	Role            *string
	Status          *string
	IsEmailVerified *bool
}

// Empty reports a patch that asks for nothing.
func (p UserPatch) Empty() bool {
	return p.Email == nil && p.FirstName == nil && p.LastName == nil &&
		p.AvatarURL == nil && p.PhoneNumber == nil && p.Role == nil &&
		p.Status == nil && p.IsEmailVerified == nil
}

// Privileged reports whether this patch touches a field only an admin may set.
// Role, status and verified-ness are authorization state: a user who could set
// their own role could make themselves an admin.
func (p UserPatch) Privileged() bool {
	return p.Role != nil || p.Status != nil || p.IsEmailVerified != nil
}

// Apply writes the present fields onto a user.
func (p UserPatch) Apply(target *User) {
	if p.Email != nil {
		target.Email = *p.Email
	}
	if p.FirstName != nil {
		target.FirstName = *p.FirstName
	}
	if p.LastName != nil {
		target.LastName = *p.LastName
	}
	if p.AvatarURL != nil {
		target.AvatarURL = p.AvatarURL
	}
	if p.PhoneNumber != nil {
		target.PhoneNumber = p.PhoneNumber
	}
	if p.Role != nil {
		target.Role = *p.Role
	}
	if p.Status != nil {
		target.Status = *p.Status
	}
	if p.IsEmailVerified != nil {
		target.IsEmailVerified = *p.IsEmailVerified
	}
}
