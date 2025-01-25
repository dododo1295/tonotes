package services

import (
	"context"
	"fmt"
	"main/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type RedisTokenBlacklist struct {
	Client *redis.Client
}

// TokenBlacklist is the global instance
var TokenBlacklist *RedisTokenBlacklist

// NewTokenBlacklist creates a new Redis-backed token blacklist
func NewTokenBlacklist(redisURL string) (*RedisTokenBlacklist, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %v", err)
	}

	client := redis.NewClient(opts)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RedisTokenBlacklist{Client: client}, nil
}

// BlacklistTokens adds both access and refresh tokens to the blacklist
func BlacklistTokens(accessToken, refreshToken string) error {
	if TokenBlacklist == nil {
		return fmt.Errorf("token blacklist not initialized")
	}
	return TokenBlacklist.blacklistTokens(accessToken, refreshToken)
}

func (tb *RedisTokenBlacklist) blacklistTokens(accessToken, refreshToken string) error {
	// Blacklist access token
	if err := tb.blacklistSingleToken(accessToken, "access"); err != nil {
		return fmt.Errorf("failed to blacklist access token: %v", err)
	}

	// Blacklist refresh token
	if err := tb.blacklistSingleToken(refreshToken, "refresh"); err != nil {
		return fmt.Errorf("failed to blacklist refresh token: %v", err)
	}

	return nil
}

// blacklistSingleToken adds a single token to the blacklist until its expiration
func (tb *RedisTokenBlacklist) blacklistSingleToken(tokenString string, tokenType string) error {
	// Parse the token to get its expiration time
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil {
		return nil // Invalid token doesn't need to be blacklisted
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
	timeUntilExpiration := time.Until(expirationTime)

	// If token is already expired, no need to blacklist
	if timeUntilExpiration <= 0 {
		return nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("blacklist:%s:%s", tokenType, tokenString)

	// Store token in Redis with expiration
	err = tb.Client.Set(ctx, key, true, timeUntilExpiration).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist token in Redis: %v", err)
	}

	fmt.Printf("Blacklisted %s token, expires: %v\n", tokenType, expirationTime)
	return nil
}

// IsTokenBlacklisted checks if a token is in the blacklist
func IsTokenBlacklisted(tokenString string) bool {
	if TokenBlacklist == nil {
		return false
	}
	return TokenBlacklist.isTokenBlacklisted(tokenString)
}

func (tb *RedisTokenBlacklist) isTokenBlacklisted(tokenString string) bool {
	ctx := context.Background()

	// Check both access and refresh token blacklists
	accessKey := fmt.Sprintf("blacklist:access:%s", tokenString)
	refreshKey := fmt.Sprintf("blacklist:refresh:%s", tokenString)

	// Use pipeline to check both keys in one round trip
	pipe := tb.Client.Pipeline()
	accessCmd := pipe.Exists(ctx, accessKey)
	refreshCmd := pipe.Exists(ctx, refreshKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Error checking token blacklist: %v\n", err)
		return false
	}

	// If token exists in either blacklist, it's blacklisted
	return accessCmd.Val() > 0 || refreshCmd.Val() > 0
}

// Cleanup removes expired tokens (Redis handles this automatically, but this method can be used for manual cleanup)
func (tb *RedisTokenBlacklist) Cleanup() error {
	return nil // Redis automatically removes expired keys
}

// Close closes the Redis connection
func (tb *RedisTokenBlacklist) Close() error {
	return tb.Client.Close()
}
