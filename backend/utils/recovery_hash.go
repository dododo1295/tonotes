package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashString creates a SHA-256 hash of the input string
func HashString(input string) string {
    hash := sha256.New()
    hash.Write([]byte(input))
    return hex.EncodeToString(hash.Sum(nil))
}
