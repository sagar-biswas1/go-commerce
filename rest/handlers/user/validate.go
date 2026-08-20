package user

import (
	db "go-commerce/database"
	"go-commerce/rest/helpers"
	"go-commerce/utils"
	"net/http"
	"regexp"
	"strings"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{7,14}$`) // E.164-ish, 8-15 digits
)

var validRoles = map[string]bool{
	"admin": true,
	"user":  true,
	"staff": true,
}

var validStatuses = map[string]bool{
	"active":    true,
	"inactive":  true,
	"suspended": true,
	"pending":   true,
}

type UserValidator struct {
	isValidUser bool
	Errors      map[string]string
}

func validateEmail(email string) (string, bool) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "email is required", false
	}
	if len(email) > 254 {
		return "email must be 254 characters or fewer", false
	}
	if !emailRegex.MatchString(email) {
		return "email is not a valid email address", false
	}
	return "", true
}

func validatePassword(password string) (string, bool) {
	if len(password) < 8 {
		return "password must be at least 8 characters", false
	}
	if len(password) > 72 {
		// bcrypt silently truncates beyond 72 bytes
		return "password must be 72 characters or fewer", false
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

func validateName(field, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return field + " is required", false
	}
	if len(name) > 100 {
		return field + " must be 100 characters or fewer", false
	}
	return "", true
}

func validatePhone(phone *string) (string, bool) {
	if phone == nil {
		return "", true // optional field
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

func validateRole(role string) (string, bool) {
	if role == "" {
		return "role is required", false
	}
	if !validRoles[role] {
		return "role must be one of: admin, user, staff", false
	}
	return "", true
}

func validateStatus(status string) (string, bool) {
	if status == "" {
		return "status is required", false
	}
	if !validStatuses[status] {
		return "status must be one of: active, inactive, suspended, pending", false
	}
	return "", true
}

func NewUserValidator() *UserValidator {
	return &UserValidator{
		Errors: make(map[string]string),
	}
}

func (v *UserValidator) IsValid() bool {
	return v.isValidUser
}

func (v *UserValidator) reset() {
	v.Errors = make(map[string]string)
	v.isValidUser = true
}

func (v *UserValidator) addError(field, msg string) {
	v.Errors[field] = msg
	v.isValidUser = false
}

func (v *UserValidator) ValidateFull(user db.User) bool {
	v.reset()

	if msg, ok := validateEmail(user.Email); !ok {
		v.addError("email", msg)
	}
	if msg, ok := validatePassword(user.Password); !ok {
		v.addError("password", msg)
	}
	if msg, ok := validateName("firstName", user.FirstName); !ok {
		v.addError("firstName", msg)
	}
	if msg, ok := validateName("lastName", user.LastName); !ok {
		v.addError("lastName", msg)
	}
	if msg, ok := validatePhone(user.PhoneNumber); !ok {
		v.addError("phoneNumber", msg)
	}
	if msg, ok := validateRole(user.Role); !ok {
		v.addError("role", msg)
	}
	if msg, ok := validateStatus(user.Status); !ok {
		v.addError("status", msg)
	}

	return v.isValidUser
}

// ValidatePartial checks only the fields present in a PATCH body. A field
// that is present but empty is still validated, so clients cannot blank out
// required fields.
func (v *UserValidator) ValidatePartial(patch UserPatch) bool {
	v.reset()

	if patch.Email != nil {
		if msg, ok := validateEmail(*patch.Email); !ok {
			v.addError("email", msg)
		}
	}
	if patch.FirstName != nil {
		if msg, ok := validateName("firstName", *patch.FirstName); !ok {
			v.addError("firstName", msg)
		}
	}
	if patch.LastName != nil {
		if msg, ok := validateName("lastName", *patch.LastName); !ok {
			v.addError("lastName", msg)
		}
	}
	if patch.PhoneNumber != nil {
		if msg, ok := validatePhone(patch.PhoneNumber); !ok {
			v.addError("phoneNumber", msg)
		}
	}
	if patch.Role != nil {
		if msg, ok := validateRole(*patch.Role); !ok {
			v.addError("role", msg)
		}
	}
	if patch.Status != nil {
		if msg, ok := validateStatus(*patch.Status); !ok {
			v.addError("status", msg)
		}
	}

	return v.isValidUser
}

func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := helpers.PathInt(r, "id")
	if err != nil {
		utils.SendError(w, "Invalid product ID", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
