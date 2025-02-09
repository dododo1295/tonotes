package repository

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	// Notes indexes
	noteIndexes = []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("user_notes_date"),
		},
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "content", Value: "text"},
				{Key: "tags", Value: "text"},
			},
			Options: options.Index().
				SetName("text_search").
				SetWeights(bson.D{
					{Key: "title", Value: 10},
					{Key: "content", Value: 5},
					{Key: "tags", Value: 3},
				}),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "is_pinned", Value: 1},
			},
			Options: options.Index().SetName("user_pinned_notes"),
		},
	}

	// Todos indexes
	todosIndexes = []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("user_todos_date"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "complete", Value: 1},
			},
			Options: options.Index().SetName("user_todos_status"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "priority", Value: 1},
			},
			Options: options.Index().SetName("user_todos_priority"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "due_date", Value: 1},
			},
			Options: options.Index().SetName("user_todos_due_date"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "reminder_at", Value: 1},
			},
			Options: options.Index().SetName("user_todos_reminder"),
		},
		{
			Keys: bson.D{
				{Key: "todo_name", Value: "text"},
				{Key: "todo_description", Value: "text"},
				{Key: "tags", Value: "text"},
			},
			Options: options.Index().
				SetName("todos_text_search").
				SetWeights(bson.D{
					{Key: "todo_name", Value: 10},
					{Key: "todo_description", Value: 5},
					{Key: "tags", Value: 3},
				}),
		},
	}

	// Users indexes
	usersIndexes = []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "username", Value: 1}},
			Options: options.Index().
				SetName("username_index").
				SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().
				SetName("user_id_index").
				SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "email", Value: 1}},
			Options: options.Index().
				SetName("email_index").
				SetUnique(true),
		},
	}

	// Sessions indexes
	sessionsIndexes = []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "session_id", Value: 1},
			},
			Options: options.Index().SetName("user_session_index"),
		},
		{
			Keys: bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().
				SetName("session_expiry_index").
				SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "is_active", Value: 1},
			},
			Options: options.Index().SetName("user_active_sessions"),
		},
	}
)

func SetupIndexes(db *mongo.Database) error {
	if db == nil {
		return fmt.Errorf("database instance is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbName := db.Name()
	log.Printf("Setting up indexes for database: %s", dbName)

	// Verify database exists
	dbs, err := db.Client().ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}
	log.Printf("Available databases: %v", dbs)

	// Create collections first
	for _, collName := range []string{"notes", "todos", "users", "sessions"} {
		log.Printf("Ensuring collection exists: %s.%s", dbName, collName)
		err := db.CreateCollection(ctx, collName)
		if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
			return fmt.Errorf("failed to create collection %s: %w", collName, err)
		}
	}

	// List collections to verify
	colls, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}
	log.Printf("Collections in %s: %v", dbName, colls)

	// Get collection references
	notesCollection := os.Getenv("NOTE_COLLECTION")
	todosCollection := os.Getenv("TODO_COLLECTION")
	usersCollection := os.Getenv("USER_COLLECTION")
	sessionsCollection := os.Getenv("SESSION_COLLECTION")

	// Create indexes with error handling
	if notesCollection == "" || todosCollection == "" || usersCollection == "" || sessionsCollection == "" {
		return fmt.Errorf("env file isn't being read & collections are not initialized")
	}

	log.Printf("Successfully created all indexes in database: %s", dbName)
	return nil
}
