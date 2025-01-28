package repository

import (
	"context"
	"fmt"
	"log"
	"main/model"
	"main/services"
	"main/utils"
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
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("SESSIONS_COLLECTION")
	return &SessionRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}

}

func (r *SessionRepo) CreateSession(session *model.Session) error {
	timer := utils.TrackDBOperation("insert", "sessions")
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if session == nil {
		utils.TrackError("database", "nil_session")
		return fmt.Errorf("session cannot be nil")
	}

	if session.SessionID == "" || session.UserID == "" {
		utils.TrackError("database", "invalid_session_data")
		return fmt.Errorf("invalid session data: missing required fields")
	}

	result, err := r.MongoCollection.InsertOne(ctx, session)
	if err != nil {
		utils.TrackError("database", "session_creation_failed")
		return fmt.Errorf("failed to create session in database: %w", err)
	}

	if result == nil {
		utils.TrackError("database", "session_creation_no_result")
		return fmt.Errorf("failed to create session: no result returned")
	}

	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.SetSession(session); err != nil {
			utils.TrackError("cache", "session_cache_set_failed")
			log.Printf("Warning: Failed to cache session: %v", err)
		}
		utils.TrackCacheOperation("session", true)

		if err := services.GlobalSessionCache.IncrementSessionVersion(session.UserID); err != nil {
			utils.TrackError("cache", "session_version_increment_failed")
			log.Printf("Warning: Failed to increment session version: %v", err)
		}
	}

	return nil
}

func (r *SessionRepo) GetSession(sessionID string) (*model.Session, error) {
	timer := utils.TrackDBOperation("find", "sessions")
	defer timer.ObserveDuration()

	if sessionID == "" {
		utils.TrackError("database", "empty_session_id")
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	if services.GlobalSessionCache != nil {
		if session, err := services.GlobalSessionCache.GetSession(sessionID); err == nil && session != nil {
			utils.TrackCacheOperation("session", true) // Cache hit
			return session, nil
		}
		utils.TrackCacheOperation("session", false) // Cache miss
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var session model.Session
	err := r.MongoCollection.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.TrackError("database", "session_not_found")
			return nil, nil
		}
		utils.TrackError("database", "session_fetch_failed")
		return nil, fmt.Errorf("failed to fetch session from database: %w", err)
	}

	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.SetSession(&session); err != nil {
			utils.TrackError("cache", "session_cache_set_failed")
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
	timer := utils.TrackDBOperation("delete", "sessions")
	defer timer.ObserveDuration()

	if sessionID == "" {
		utils.TrackError("database", "empty_session_id")
		return fmt.Errorf("sessionID cannot be empty")
	}

	session, err := r.GetSession(sessionID)
	if err != nil {
		utils.TrackError("database", "session_fetch_failed")
		return fmt.Errorf("failed to fetch session for deletion: %w", err)
	}
	if session == nil {
		utils.TrackError("database", "session_not_found")
		return fmt.Errorf("session not found")
	}

	if session.Protected {
		utils.TrackError("database", "protected_session_deletion_attempt")
		return fmt.Errorf("cannot delete protected session")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := r.MongoCollection.DeleteOne(ctx, bson.M{"session_id": sessionID})
	if err != nil {
		utils.TrackError("database", "session_deletion_failed")
		return fmt.Errorf("failed to delete session from database: %w", err)
	}

	if result.DeletedCount == 0 {
		utils.TrackError("database", "session_not_found")
		return fmt.Errorf("session not found")
	}

	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.DeleteSession(sessionID); err != nil {
			utils.TrackError("cache", "session_cache_delete_failed")
			log.Printf("Warning: Failed to delete session from cache: %v", err)
		}

		if err := services.GlobalSessionCache.IncrementSessionVersion(session.UserID); err != nil {
			utils.TrackError("cache", "session_version_increment_failed")
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
	timer := utils.TrackDBOperation("find", "sessions")
	defer timer.ObserveDuration()

	if userID == "" {
		utils.TrackError("database", "empty_user_id")
		return nil, fmt.Errorf("userID cannot be empty")
	}

	if services.GlobalSessionCache != nil {
		sessions, isStale, err := services.GlobalSessionCache.GetUserSessions(userID)
		if err == nil && sessions != nil && !isStale {
			utils.TrackCacheOperation("user_sessions", true)
			return sessions, nil
		}
		utils.TrackCacheOperation("user_sessions", false)

		needsRefresh, _ := services.GlobalSessionCache.NeedsRefresh(userID)
		if isStale || needsRefresh {
			sessions, err = r.fetchAndCacheActiveSessions(userID)
			if err != nil {
				if isStale {
					return sessions, nil
				}
				utils.TrackError("database", "session_fetch_failed")
				return nil, err
			}
			return sessions, nil
		}
	}

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
