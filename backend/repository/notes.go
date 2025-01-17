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

func (r *NotesRepo) TogglePin(noteID string, userID string) error {
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
			"is_pinned":  !note.IsPinned,
			"updated_at": time.Now(),
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

// GetPinnedNotes retrieves all pinned notes for a user
func (r *NotesRepo) GetPinnedNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{
			"user_id":     userID,
			"is_pinned":   true,
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

func (r *NotesRepo) SearchByTags(userID string, tags []string) ([]*model.Notes, error) {
	filter := bson.M{
		"user_id":     userID,
		"is_archived": false,
		"tags": bson.M{
			"$in": tags, // matches any of the tags in the array
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

// GetAllTags retrieves all unique tags used by a user
func (r *NotesRepo) GetAllTags(userID string) ([]string, error) {
	// Using MongoDB's distinct command to get unique tags
	tags, err := r.MongoCollection.Distinct(
		context.Background(),
		"tags",
		bson.M{"user_id": userID},
	)
	if err != nil {
		return nil, err
	}

	// Convert interface{} array to string array
	stringTags := make([]string, 0)
	for _, tag := range tags {
		if strTag, ok := tag.(string); ok {
			stringTags = append(stringTags, strTag)
		}
	}

	return stringTags, nil
}

func (r *NotesRepo) SearchByTagsWithOptions(userID string, tags []string, matchAll bool) ([]*model.Notes, error) {
	var filter bson.M
	if matchAll {
		// Match all tags (AND operation)
		filter = bson.M{
			"user_id":     userID,
			"is_archived": false,
			"tags": bson.M{
				"$all": tags,
			},
		}
	} else {
		// Match any tags (OR operation)
		filter = bson.M{
			"user_id":     userID,
			"is_archived": false,
			"tags": bson.M{
				"$in": tags,
			},
		}
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

func (r *NotesRepo) CountNotesByTag(userID string) (map[string]int, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"user_id":     userID,
				"is_archived": false,
			},
		},
		{
			"$unwind": "$tags",
		},
		{
			"$group": bson.M{
				"_id":   "$tags",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.MongoCollection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	type tagCount struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []tagCount
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, result := range results {
		tagCounts[result.ID] = result.Count
	}

	return tagCounts, nil
}
