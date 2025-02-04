package test

import (
	"context"
	"fmt"
	"main/services"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) {
	// Create a mock Redis client using a test server
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
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

func TestRedisBlacklist(t *testing.T) {
	setupTestRedis(t)
	defer services.TokenBlacklist.Close()

	// First, test direct Redis operations
	t.Run("Direct Redis Operations", func(t *testing.T) {
		ctx := context.Background()

		// Clear Redis
		if err := services.TokenBlacklist.Client.FlushDB(ctx).Err(); err != nil {
			t.Fatalf("Failed to clear Redis: %v", err)
		}

		// Test direct set
		testKey := "test:key"
		testValue := "test-value"
		err := services.TokenBlacklist.Client.Set(ctx, testKey, testValue, time.Hour).Err()
		if err != nil {
			t.Fatalf("Failed to set test key: %v", err)
		}

		// Verify the key exists
		exists, err := services.TokenBlacklist.Client.Exists(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("Failed to check key existence: %v", err)
		}
		if exists == 0 {
			t.Fatal("Test key not found in Redis")
		}

		// Check the value
		val, err := services.TokenBlacklist.Client.Get(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("Failed to get test key: %v", err)
		}
		if val != testValue {
			t.Errorf("Expected value %s, got %s", testValue, val)
		}

		// Check TTL
		ttl, err := services.TokenBlacklist.Client.TTL(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("Failed to get TTL: %v", err)
		}
		if ttl <= 0 {
			t.Errorf("Expected positive TTL, got %v", ttl)
		}
	})

	// Now test the blacklist implementation
	t.Run("Blacklist Implementation", func(t *testing.T) {
		ctx := context.Background()

		// Clear Redis
		if err := services.TokenBlacklist.Client.FlushDB(ctx).Err(); err != nil {
			t.Fatalf("Failed to clear Redis: %v", err)
		}

		// Test tokens
		accessToken := "test-access-token"
		refreshToken := "test-refresh-token"

		// Try to blacklist
		err := services.BlacklistTokens(accessToken, refreshToken)
		if err != nil {
			t.Fatalf("Failed to blacklist tokens: %v", err)
		}

		// Debug: List all keys
		keys, err := services.TokenBlacklist.Client.Keys(ctx, "*").Result()
		t.Logf("All keys in Redis after blacklisting: %v", keys)

		// Check each key directly
		accessKey := fmt.Sprintf("blacklist:access:%s", accessToken)
		refreshKey := fmt.Sprintf("blacklist:refresh:%s", refreshToken)

		// Debug access token
		accessExists, err := services.TokenBlacklist.Client.Exists(ctx, accessKey).Result()
		t.Logf("Access key %s exists: %v", accessKey, accessExists > 0)
		if accessExists > 0 {
			val, _ := services.TokenBlacklist.Client.Get(ctx, accessKey).Result()
			ttl, _ := services.TokenBlacklist.Client.TTL(ctx, accessKey).Result()
			t.Logf("Access token value: %s, TTL: %v", val, ttl)
		}

		// Debug refresh token
		refreshExists, err := services.TokenBlacklist.Client.Exists(ctx, refreshKey).Result()
		t.Logf("Refresh key %s exists: %v", refreshKey, refreshExists > 0)
		if refreshExists > 0 {
			val, _ := services.TokenBlacklist.Client.Get(ctx, refreshKey).Result()
			ttl, _ := services.TokenBlacklist.Client.TTL(ctx, refreshKey).Result()
			t.Logf("Refresh token value: %s, TTL: %v", val, ttl)
		}
	})
}
