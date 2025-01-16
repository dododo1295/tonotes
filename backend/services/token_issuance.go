package services

import (
	"fmt"
	"time"

	"main/utils"

	"github.com/golang-jwt/jwt/v5" // Update the import here
)

// GenerateJWT generates a JWT token for the user with their ID and expiration time
func GenerateToken(userID string) (string, error) {
	// Use the loaded expiration time from the utils package
	expirationTime := time.Now().Add(time.Duration(utils.JWTExpirationTime) * time.Second)

	// Claims for the JWT
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expirationTime.Unix(),
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
	}

	// Generate the token using the JWT secret from the utils package
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		return "invalid token", err
	}

	return signedToken, nil
}

type TokenClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func ValidateToken(tokenString string) (string, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %v", err)
	}

	// Check if token is blacklisted
	if IsTokenBlacklisted(tokenString) {
		return "", fmt.Errorf("token is blacklisted")
	}

	// Get the claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				return "", fmt.Errorf("token has expired")
			}
		}

		// Get and return the user ID
		if userID, ok := claims["user_id"].(string); ok {
			return userID, nil
		}
		return "", fmt.Errorf("invalid claims")
	}

	return "", fmt.Errorf("invalid token")
}

func GenerateRefreshToken(userID string) (string, error) {
	expirationTime := time.Now().Add(time.Duration(utils.RefreshTokenExpirationTime) * time.Second)
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expirationTime.Unix(),
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
		"type":    "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		return "invalid refresh", err
	}
	return signedToken, nil
}

func ValidateRefreshToken(tokenString string) (string, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil {
		return "", err
	}

	// Get the claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check if it's a refresh token
		if tokenType, ok := claims["type"].(string); !ok || tokenType != "refresh" {
			return "", fmt.Errorf("invalid token type")
		}

		// Check expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				return "", fmt.Errorf("token has expired")
			}
		}

		// Get and return the user ID
		if userID, ok := claims["user_id"].(string); ok {
			return userID, nil
		}
		return "", fmt.Errorf("invalid claims")
	}

	return "", fmt.Errorf("invalid refresh")
}
