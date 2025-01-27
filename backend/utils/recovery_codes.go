package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	RecoveryCodeLength = 8
	NumRecoveryCodes   = 10
)

// GenerateRecoveryCodes generates a set of random recovery codes
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, NumRecoveryCodes)

	for i := 0; i < NumRecoveryCodes; i++ {
		// Generate random bytes
		bytes := make([]byte, RecoveryCodeLength/2)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}

		// Convert to hex string and format
		code := hex.EncodeToString(bytes)
		code = strings.ToUpper(code)
		// Insert hyphen in middle for readability
		code = code[:4] + "-" + code[4:]
		codes[i] = code
	}

	return codes, nil
}

// HashRecoveryCodes hashes the recovery codes for storage
func HashRecoveryCodes(codes []string) []string {
	hashedCodes := make([]string, len(codes))
	for i, code := range codes {
		hashedCodes[i] = HashString(code)
	}
	return hashedCodes
}
