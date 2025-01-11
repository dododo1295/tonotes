package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("failed to generate salt")
	}

	const (
		memory      = 64 * 1024
		iterations  = 3
		parallelism = 2
		keyLength   = 32
	)
	// Hashes Password
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	// Encodes hash and salt
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)

	// combine
	return encodedSalt + "$" + encodedHash, nil
}
