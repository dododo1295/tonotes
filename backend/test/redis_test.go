package test

import (
	"context"
	"main/services"
	"testing"

	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) {
	// Create a mock Redis client using a test server
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use your test Redis instance
		DB:   1,                // Use a different DB for testing
	})

	// Create a new TokenBlacklist instance
	blacklist := &services.RedisTokenBlacklist{
		Client: client,
	}

	// Set the global TokenBlacklist instance
	services.TokenBlacklist = blacklist

	// Clear the test database
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush test Redis DB: %v", err)
	}

	// Ensure connection is working
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
}
