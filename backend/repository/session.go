package repository

import (
	"context"
	"fmt"
	"log"
	"main/model"
	"main/services"
	"os"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SessionRepo struct {
	MongoCollection *mongo.Collection
}

func GetSessionRepo(client *mongo.Client) *SessionRepo {
	return &SessionRepo{
		MongoCollection: client.Database(os.Getenv("MONGO_DB")).Collection("sessions"),
	}
}

func (r *SessionRepo) CreateSession(session *model.Session) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Validate session data
	if session.SessionID == "" || session.UserID == "" {
		return fmt.Errorf("invalid session data: missing required fields")
	}

	// First create in database
	result, err := r.MongoCollection.InsertOne(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to create session in database: %w", err)
	}

	// Verify the insertion
	if result == nil {
		return fmt.Errorf("failed to create session: no result returned")
	}

	// Cache the new session if caching is enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.SetSession(session); err != nil {
			log.Printf("Warning: Failed to cache session: %v", err)
		}

		// Invalidate user's sessions cache
		if err := services.GlobalSessionCache.IncrementSessionVersion(session.UserID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	return nil
}

func (r *SessionRepo) GetSession(sessionID string) (*model.Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	// Try cache first if enabled
	if services.GlobalSessionCache != nil {
		if session, err := services.GlobalSessionCache.GetSession(sessionID); err == nil && session != nil {
			return session, nil
		}
	}

	// Cache miss or disabled - get from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var session model.Session
	err := r.MongoCollection.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch session from database: %w", err)
	}

	// Cache the result if caching is enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.SetSession(&session); err != nil {
			log.Printf("Warning: Failed to cache session: %v", err)
		}
	}

	return &session, nil
}

func (r *SessionRepo) UpdateSession(session *model.Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"last_activity_at": time.Now(),
			"is_active":        session.IsActive,
			"expires_at":       session.ExpiresAt,
			"device_info":      session.DeviceInfo,
			"ip_address":       session.IPAddress,
		},
	}

	result, err := r.MongoCollection.UpdateOne(
		ctx,
		bson.M{"session_id": session.SessionID},
		update,
	)
	if err != nil {
		return fmt.Errorf("failed to update session in database: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("session not found")
	}

	// Update cache if enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.SetSession(session); err != nil {
			log.Printf("Warning: Failed to update session cache: %v", err)
		}

		if err := services.GlobalSessionCache.IncrementSessionVersion(session.UserID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	return nil
}

func (r *SessionRepo) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	// Get session first to check protected status and get userID
	session, err := r.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to fetch session for deletion: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Check protected status
	if session.Protected {
		return fmt.Errorf("cannot delete protected session")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := r.MongoCollection.DeleteOne(ctx, bson.M{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("failed to delete session from database: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("session not found")
	}

	// Update cache if enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.DeleteSession(sessionID); err != nil {
			log.Printf("Warning: Failed to delete session from cache: %v", err)
		}

		if err := services.GlobalSessionCache.IncrementSessionVersion(session.UserID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	return nil
}

func (r *SessionRepo) DeleteUserSessions(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := r.MongoCollection.DeleteMany(ctx, bson.M{"user_id": userID})
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	// Invalidate cache if enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.IncrementSessionVersion(userID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	log.Printf("Deleted %d sessions for user %s", result.DeletedCount, userID)
	return nil
}

func (r *SessionRepo) GetUserActiveSessions(userID string) ([]*model.Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	// Try cache first if enabled
	if services.GlobalSessionCache != nil {
		sessions, isStale, err := services.GlobalSessionCache.GetUserSessions(userID)
		if err == nil && sessions != nil && !isStale {
			return sessions, nil
		}

		// If cache is stale or needs refresh, fetch from database
		needsRefresh, _ := services.GlobalSessionCache.NeedsRefresh(userID)
		if isStale || needsRefresh {
			sessions, err = r.fetchAndCacheActiveSessions(userID)
			if err != nil {
				if isStale {
					return sessions, nil // Return stale data if fresh fetch fails
				}
				return nil, err
			}
			return sessions, nil
		}
	}

	// Cache disabled or complete cache miss - fetch from database
	return r.fetchAndCacheActiveSessions(userID)
}

func (r *SessionRepo) fetchAndCacheActiveSessions(userID string) ([]*model.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.M{"last_activity_at": -1})
	cursor, err := r.MongoCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active sessions: %w", err)
	}
	defer cursor.Close(ctx)

	var sessions []*model.Session
	if err = cursor.All(ctx, &sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	// Cache the fresh data if caching is enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.CacheUserSessions(userID, sessions); err != nil {
			log.Printf("Warning: Failed to cache user sessions: %v", err)
		}
	}

	return sessions, nil
}

func (r *SessionRepo) EndAllUserSessions(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"is_active":        false,
			"last_activity_at": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateMany(
		ctx,
		bson.M{"user_id": userID, "is_active": true},
		update,
	)
	if err != nil {
		return fmt.Errorf("failed to end user sessions: %w", err)
	}

	// Invalidate cache if enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.IncrementSessionVersion(userID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	log.Printf("Ended %d active sessions for user %s", result.ModifiedCount, userID)
	return nil
}

func (r *SessionRepo) EndLeastActiveSession(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch active sessions
	sessions, err := r.GetUserActiveSessions(userID)
	if err != nil {
		return fmt.Errorf("failed to fetch active sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no active sessions found")
	}

	// Sort sessions by last activity time
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActivityAt.Before(sessions[j].LastActivityAt)
	})

	// End the least active session
	leastActive := sessions[0]
	update := bson.M{
		"$set": bson.M{
			"is_active":        false,
			"last_activity_at": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(
		ctx,
		bson.M{"session_id": leastActive.SessionID},
		update,
	)
	if err != nil {
		return fmt.Errorf("failed to end least active session: %w", err)
	}

	if result.ModifiedCount == 0 {
		return fmt.Errorf("failed to end session: session not found")
	}

	// Update cache if enabled
	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.DeleteSession(leastActive.SessionID); err != nil {
			log.Printf("Warning: Failed to delete session from cache: %v", err)
		}

		if err := services.GlobalSessionCache.IncrementSessionVersion(userID); err != nil {
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	return nil
}

func (r *SessionRepo) CountActiveSessions(userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("userID cannot be empty")
	}

	// Try cache first if enabled
	if services.GlobalSessionCache != nil {
		sessions, isStale, err := services.GlobalSessionCache.GetUserSessions(userID)
		if err == nil && !isStale && sessions != nil {
			count := 0
			now := time.Now()
			for _, session := range sessions {
				if session.IsActive && session.ExpiresAt.After(now) {
					count++
				}
			}
			return count, nil
		}
	}

	// Cache miss or disabled - count from database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := r.MongoCollection.CountDocuments(
		ctx,
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count active sessions: %w", err)
	}

	return int(count), nil
}
