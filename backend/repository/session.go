package repository

import (
	"context"
	"fmt"
	"main/model"
	"os"
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
	_, err := r.MongoCollection.InsertOne(context.Background(), session)
	return err
}

func (r *SessionRepo) GetSession(sessionID string) (*model.Session, error) {
	var session model.Session
	err := r.MongoCollection.FindOne(context.Background(),
		bson.M{"session_id": sessionID}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepo) UpdateSession(session *model.Session) error {
	_, err := r.MongoCollection.UpdateOne(
		context.Background(),
		bson.M{"session_id": session.SessionID},
		bson.M{"$set": bson.M{
			"last_activity_at": time.Now(),
			"is_active":        session.IsActive,
		}},
	)
	return err
}

func (r *SessionRepo) DeleteSession(sessionID string) error {
	_, err := r.MongoCollection.DeleteOne(
		context.Background(),
		bson.M{"session_id": sessionID},
	)
	return err
}

func (r *SessionRepo) DeleteUserSessions(userID string) error {
	_, err := r.MongoCollection.DeleteMany(
		context.Background(),
		bson.M{"user_id": userID},
	)
	return err
}

func (r *SessionRepo) GetUserActiveSessions(userID string) ([]*model.Session, error) {
	var sessions []*model.Session
	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *SessionRepo) EndAllUserSessions(userID string) error {
	_, err := r.MongoCollection.UpdateMany(
		context.Background(),
		bson.M{"user_id": userID},
		bson.M{
			"$set": bson.M{
				"is_active":        false,
				"last_activity_at": time.Now(),
			},
		},
	)
	return err
}
func (sr *SessionRepo) GetSessionByID(sessionID string) (*model.Session, error) {
	var session model.Session
	filter := bson.M{"session_id": sessionID}

	err := sr.MongoCollection.FindOne(context.Background(), filter).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
}

func (r *SessionRepo) CountActiveSessions(userID string) (int, error) {
	count, err := r.MongoCollection.CountDocuments(
		context.Background(),
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		},
	)
	return int(count), err
}

// on session creation, if the user has more than the maximum number of active sessions, end the least active session
func (r *SessionRepo) EndLeastActiveSession(userID string) error {
	ctx := context.Background()

	// Find the least recently active session
	var leastActiveSession model.Session
	err := r.MongoCollection.FindOne(
		ctx,
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		},
		options.FindOne().
			SetSort(bson.D{{Key: "last_activity_at", Value: 1}}),
	).Decode(&leastActiveSession)

	if err != nil {
		return fmt.Errorf("failed to find least active session: %w", err)
	}

	// End the session
	_, err = r.MongoCollection.UpdateOne(
		ctx,
		bson.M{"session_id": leastActiveSession.SessionID},
		bson.M{
			"$set": bson.M{
				"is_active":        false,
				"last_activity_at": time.Now(),
			},
		},
	)
	return err
}
