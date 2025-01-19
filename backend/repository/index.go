package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SetupIndexes(db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize collections
	notesCollection := db.Collection("notes")
	todosCollection := db.Collection("todos")

	// Define indexes
	noteIndexes := []mongo.IndexModel{
		// Basic user-date index
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName("user_notes_date").
				SetUnique(false),
		},
		// User ID index
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().
				SetName("user_id_index"),
		},
		// Pinned notes index
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "is_pinned", Value: 1},
				{Key: "pinned_position", Value: 1},
			},
			Options: options.Index().
				SetName("user_pinned_notes_order").
				SetUnique(false),
		},
		// Text search index
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "content", Value: "text"},
				{Key: "tags", Value: "text"},
			},
			Options: options.Index().
				SetName("text_search").
				SetDefaultLanguage("english").
				SetWeights(bson.D{
					{Key: "title", Value: 10},
					{Key: "content", Value: 5},
					{Key: "tags", Value: 3},
				}),
		},
		// Tags index
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "tags", Value: 1},
			},
			Options: options.Index().
				SetName("user_tags"),
		},
		// Archive index
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "is_archived", Value: 1},
				{Key: "updated_at", Value: -1},
			},
			Options: options.Index().
				SetName("user_archived_notes"),
		},
	}

	todosIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName("user_todos_date").
				SetUnique(false),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().
				SetName("user_id_index"),
		},
	}

	// Create indexes for notes
	_, err := notesCollection.Indexes().CreateMany(ctx, noteIndexes)
	if err != nil {
		return fmt.Errorf("failed to create notes indexes: %w", err)
	}

	// Create indexes for todos
	_, err = todosCollection.Indexes().CreateMany(ctx, todosIndexes)
	if err != nil {
		return fmt.Errorf("failed to create todos indexes: %w", err)
	}

	log.Println("Successfully created all indexes")
	return nil
}
