package services

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"main/utils"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Constants for Argon2 parameters
const (
	memory      = 64 * 1024
	iterations  = 3
	parallelism = 2
	keyLength   = 32
)

func HashPassword(password string) (string, error) {
	if !utils.ValidatePassword(password) {
		return "", errors.New("password must be at least 6 characters and contain at least 2 numbers and at least 2 special characters")
	}

	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("failed to generate salt")
	}

	// Hash the password with Argon2
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)

	// Encode salt and hash separately
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	// Combine with $ separator
	return encodedSalt + "$" + encodedHash, nil
}

// VerifyPassword verifies if the provided password matches the stored hash
func VerifyPassword(storedPassword, providedPassword string) (bool, error) {
	// Split the stored password into salt and hash
	parts := strings.Split(storedPassword, "$")
	if len(parts) != 2 {
		return false, errors.New("invalid stored password format")
	}

	// Decode the stored salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, err
	}

	storedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, err
	}

	// Hash the provided password with the same salt and parameters
	computedHash := argon2.IDKey([]byte(providedPassword), salt, iterations, memory, parallelism, keyLength)

	// Compare the computed hash with the stored hash
	return bytes.Equal(computedHash, storedHash), nil
}

// ComparePasswords compares a stored password hash with a plain-text password
// Returns true if they match, false otherwise
func ComparePasswords(storedHash, plainPassword string) bool {
	match, err := VerifyPassword(storedHash, plainPassword)
	if err != nil {
		return false
	}
	return match
}

// CheckHashes compares two hashes securely
func CheckHashes(a, b []byte) bool {
	return bytes.Equal(a, b)
}
