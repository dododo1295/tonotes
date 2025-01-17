package repository

import (
	"context"
	"main/model"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
