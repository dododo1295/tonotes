package utils

import (
	"unicode"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func RegisterCustomValidators(v *validator.Validate) {
	v.RegisterValidation("password", ValidatePasswordRule)
}

var Validate *validator.Validate

func InitValidator() {
	Validate = validator.New()
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", ValidatePasswordRule)
	}
}

func ValidatePasswordRule(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return ValidatePassword(password) // Use your existing password validation
}

func ValidatePassword(password string) bool {
	// Password must:
	// - Be at least 6 characters long
	// - Contain at least one number
	// - Contain at least one special character

	hasNumber := false
	hasSpecial := false

	if len(password) < 6 {
		return false
	}

	for _, char := range password {
		switch {
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasNumber && hasSpecial
}
