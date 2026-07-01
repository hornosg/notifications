package validator

import (
	"regexp"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type EmailValidator struct{}

func NewEmailValidator() *EmailValidator {
	return &EmailValidator{}
}

func (v *EmailValidator) IsValid(email string) bool {
	return emailRegex.MatchString(email)
}

// IsValidEmail verifica si una dirección de email es válida (función legacy)
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
