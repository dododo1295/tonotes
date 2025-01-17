package repository

import (
	"context"
	"errors"
	"main/model"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NotesRepo struct {
	MongoCollection *mongo.Collection
}

func GetNotesRepo(client *mongo.Client) *NotesRepo {
	return &NotesRepo{
		MongoCollection: client.Database(os.Getenv("MONGO_DB")).Collection("notes"),
	}
}

// CreateNote creates a new note
func (r *NotesRepo) CreateNote(note *model.Notes) error {
	if note.UserID == "" {
		return errors.New("user ID is required")
	}

	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()

	_, err := r.MongoCollection.InsertOne(context.Background(), note)
	return err
}

// GetUserNotes retrieves all notes for a user
func (r *NotesRepo) GetUserNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID, "is_archived": false}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// GetNote retrieves a specific note
func (r *NotesRepo) GetNote(noteID string, userID string) (*model.Notes, error) {
	var note model.Notes
	err := r.MongoCollection.FindOne(context.Background(),
		bson.M{"_id": noteID, "user_id": userID}).Decode(&note)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("note not found")
		}
		return nil, err
	}
	return &note, nil
}

// UpdateNote updates a specific note
func (r *NotesRepo) UpdateNote(noteID string, userID string, updates *model.Notes) error {
	updates.UpdatedAt = time.Now()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"title":      updates.Title,
			"content":    updates.Content,
			"tags":       updates.Tags,
			"updated_at": updates.UpdatedAt,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("note not found")
	}

	return nil
}

// DeleteNote deletes a specific note
func (r *NotesRepo) DeleteNote(noteID string, userID string) error {
	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	result, err := r.MongoCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("note not found")
	}

	return nil
}

func (r *NotesRepo) ArchiveNote(noteID string, userID string) error {
	var note model.Notes
	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&note)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"is_archived": !note.IsArchived,
			"updated_at":  time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("note not found")
	}

	return nil
}

// GetArchivedNotes retrieves all archived notes for a user
func (r *NotesRepo) GetArchivedNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID, "is_archived": true}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// GetFavoriteNotes retrieves all favorite notes for a user
func (r *NotesRepo) GetFavoriteNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{
			"user_id":     userID,
			"is_favorite": true,
			"is_archived": false,
		}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// SearchNotes searches notes by title or content
func (r *NotesRepo) SearchNotes(userID string, query string) ([]*model.Notes, error) {
	filter := bson.M{
		"user_id":     userID,
		"is_archived": false,
		"$or": []bson.M{
			{"title": bson.M{"$regex": query, "$options": "i"}},
			{"content": bson.M{"$regex": query, "$options": "i"}},
			{"tags": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// CountUserNotes counts the number of notes for a user
func (r *NotesRepo) CountUserNotes(userID string) (int, error) {
	count, err := r.MongoCollection.CountDocuments(context.Background(),
		bson.M{"user_id": userID})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
