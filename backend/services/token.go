package services

import (
	"time"

	"main/utils"

	"github.com/golang-jwt/jwt/v5" // Update the import here
)

// GenerateJWT generates a JWT token for the user with their ID and expiration time
func GenerateJWT(userID string) (string, error) {
	// Use the loaded expiration time from the utils package
	expirationTime := time.Now().Add(time.Duration(utils.JWTExpirationTime) * time.Second)

	// Claims for the JWT
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expirationTime.Unix(),
	}

	// Generate the token using the JWT secret from the utils package
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
