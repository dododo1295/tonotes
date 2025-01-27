package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"main/model"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionCache struct {
	client    *redis.Client
	cacheLock sync.RWMutex
}

type SessionCacheEntry struct {
	Sessions  []*model.Session `json:"sessions"`
	Version   int64            `json:"version"`
	UpdatedAt time.Time        `json:"updated_at"`
}

var GlobalSessionCache *SessionCache

// NewSessionCache creates and initializes a new session cache
func NewSessionCache(redisURL string) (*SessionCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %v", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &SessionCache{
		client:    client,
		cacheLock: sync.RWMutex{},
	}, nil
}

// SetSession caches an individual session
func (sc *SessionCache) SetSession(session *model.Session) error {
	if session == nil {
		return fmt.Errorf("cannot cache nil session")
	}

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	ctx := context.Background()
	key := fmt.Sprintf("session:%s", session.SessionID)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	// Calculate TTL based on session expiry
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session has already expired")
	}

	// Store with TTL matching session expiry
	if err := sc.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache session: %v", err)
	}

	return nil
}

// GetSession retrieves a session from cache
func (sc *SessionCache) GetSession(sessionID string) (*model.Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()

	ctx := context.Background()
	key := fmt.Sprintf("session:%s", sessionID)

	data, err := sc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session from cache: %v", err)
	}

	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %v", err)
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		sc.DeleteSession(sessionID)
		return nil, nil
	}

	return &session, nil
}

// CacheUserSessions stores all active sessions for a user
func (sc *SessionCache) CacheUserSessions(userID string, sessions []*model.Session) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	ctx := context.Background()
	key := fmt.Sprintf("user_sessions:%s", userID)

	entry := SessionCacheEntry{
		Sessions:  sessions,
		Version:   time.Now().UnixNano(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %v", err)
	}

	// Cache for 5 minutes
	if err := sc.client.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to cache user sessions: %v", err)
	}

	return nil
}

// GetUserSessions retrieves all cached sessions for a user
func (sc *SessionCache) GetUserSessions(userID string) ([]*model.Session, bool, error) {
	if userID == "" {
		return nil, false, fmt.Errorf("userID cannot be empty")
	}

	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()

	ctx := context.Background()
	key := fmt.Sprintf("user_sessions:%s", userID)

	data, err := sc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil // Cache miss
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get user sessions from cache: %v", err)
	}

	var entry SessionCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal sessions: %v", err)
	}

	// Check if cache is stale (older than 30 seconds)
	isStale := time.Since(entry.UpdatedAt) > 30*time.Second

	return entry.Sessions, isStale, nil
}

// DeleteSession removes a session from cache
func (sc *SessionCache) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	ctx := context.Background()
	key := fmt.Sprintf("session:%s", sessionID)

	if err := sc.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from cache: %v", err)
	}

	return nil
}

// IncrementSessionVersion increments the version counter for user's sessions
func (sc *SessionCache) IncrementSessionVersion(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	ctx := context.Background()
	key := fmt.Sprintf("user_sessions_version:%s", userID)

	if err := sc.client.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to increment session version: %v", err)
	}

	return nil
}

// NeedsRefresh checks if the cache needs to be refreshed
func (sc *SessionCache) NeedsRefresh(userID string) (bool, error) {
	if userID == "" {
		return true, fmt.Errorf("userID cannot be empty")
	}

	ctx := context.Background()
	versionKey := fmt.Sprintf("user_sessions_version:%s", userID)
	cachedVersion, err := sc.client.Get(ctx, versionKey).Int64()
	if err == redis.Nil {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("failed to get session version: %v", err)
	}

	sessions, _, err := sc.GetUserSessions(userID)
	if err != nil {
		return true, fmt.Errorf("failed to get user sessions: %v", err)
	}
	if sessions == nil {
		return true, nil
	}

	return cachedVersion > time.Now().UnixNano(), nil
}

// RefreshCache refreshes the session cache for a specific user
func (sc *SessionCache) RefreshCache(repo interface{}) error {
	// This method should be implemented based on your repository interface
	// It should fetch fresh session data from the database and update the cache
	return nil
}

// CleanupExpiredSessions removes expired sessions from the cache
func (sc *SessionCache) CleanupExpiredSessions() error {
	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	ctx := context.Background()
	pattern := "session:*"

	var cursor uint64
	for {
		keys, newCursor, err := sc.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %v", err)
		}

		for _, key := range keys {
			data, err := sc.client.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}

			var session model.Session
			if err := json.Unmarshal(data, &session); err != nil {
				continue
			}

			if time.Now().After(session.ExpiresAt) {
				sc.client.Del(ctx, key)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// StartCleanupTask starts a background task to clean up expired sessions
func (sc *SessionCache) StartCleanupTask() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			if err := sc.CleanupExpiredSessions(); err != nil {
				log.Printf("Error cleaning up expired sessions: %v", err)
			}
		}
	}()
}

func (sc *SessionCache) IsConnected() bool {
	if sc == nil || sc.client == nil { // Changed from sc.Client to sc.client
		return false
	}
	// Check if connection is alive
	ctx := context.Background()
	return sc.client.Ping(ctx).Err() == nil
}

// Close closes the Redis connection
func (sc *SessionCache) Close() error {
	return sc.client.Close()
}

// Helper function to check if a key exists in Redis
func (sc *SessionCache) keyExists(ctx context.Context, key string) (bool, error) {
	exists, err := sc.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %v", err)
	}
	return exists > 0, nil
}

// Helper function to set key with retry logic
func (sc *SessionCache) setWithRetry(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var err error
	for i := 0; i < 3; i++ {
		err = sc.client.Set(ctx, key, value, ttl).Err()
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return fmt.Errorf("failed to set key after retries: %v", err)
}
