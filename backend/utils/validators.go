package utils

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

func RegisterCustomValidators(v *validator.Validate) {
	v.RegisterValidation("password", ValidatePasswordRule)
}

func ValidatePasswordRule(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return ValidatePassword(password) // Use your existing password validation
}

func ValidatePassword(password string) bool {
	// Check minimum length
	if len(password) < 6 {
		return false
	}

	// Check for at least 1 uppercase and 1 lowercase
	upperCase := regexp.MustCompile(`[A-Z]`)
	lowerCase := regexp.MustCompile(`[a-z]`)

	// Check for at least 2 numbers
	numbers := regexp.MustCompile(`[0-9]`)
	numberMatches := numbers.FindAllString(password, -1)

	// Check for at least 2 special characters
	special := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`)
	specialMatches := special.FindAllString(password, -1)

	return upperCase.MatchString(password) &&
		lowerCase.MatchString(password) &&
		len(numberMatches) >= 2 &&
		len(specialMatches) >= 2
}
