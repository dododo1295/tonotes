package services

import (
	"fmt"
	"main/utils"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Simple in-memory blacklist (you might want to use Redis in production)
var (
	blacklistedTokens = make(map[string]time.Time)
	blacklistMutex    sync.RWMutex
)

// BlacklistTokens adds both access and refresh tokens to the blacklist
func BlacklistTokens(accessToken, refreshToken string) error {
	// Blacklist access token
	if err := blacklistSingleToken(accessToken, "access"); err != nil {
		return fmt.Errorf("failed to blacklist access token: %v", err)
	}

	// Blacklist refresh token
	if err := blacklistSingleToken(refreshToken, "refresh"); err != nil {
		return fmt.Errorf("failed to blacklist refresh token: %v", err)
	}

	return nil
}

// blacklistSingleToken adds a single token to the blacklist until its expiration
func blacklistSingleToken(tokenString string, tokenType string) error {
	// Parse the token to get its expiration time
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil {
		// Instead of returning the error, return nil since an invalid token
		// doesn't need to be blacklisted
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil
	}

	expirationTime := time.Unix(int64(exp), 0)

	// Add token to blacklist
	blacklistMutex.Lock()
	blacklistedTokens[tokenString] = expirationTime
	fmt.Printf("Blacklisted %s token, expires: %v\n", tokenType, expirationTime)
	blacklistMutex.Unlock()

	// Schedule cleanup after expiration
	go func() {
		time.Sleep(time.Until(expirationTime))
		blacklistMutex.Lock()
		delete(blacklistedTokens, tokenString)
		blacklistMutex.Unlock()
	}()

	return nil
}

// IsTokenBlacklisted checks if a token is in the blacklist
func IsTokenBlacklisted(tokenString string) bool {
	blacklistMutex.RLock()
	defer blacklistMutex.RUnlock()

	expirationTime, exists := blacklistedTokens[tokenString]
	if !exists {
		return false
	}

	// If token is in blacklist but expired, remove it
	if time.Now().After(expirationTime) {
		go func() {
			blacklistMutex.Lock()
			delete(blacklistedTokens, tokenString)
			blacklistMutex.Unlock()
		}()
		return false
	}

	return true
}
