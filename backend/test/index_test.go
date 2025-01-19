package test

import (
	"context"
	"main/repository"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestSetupIndexes(t *testing.T) {
	// Set test environment
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", "tonotes_test")

	// Connect to MongoDB
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	db := client.Database("tonotes_test")

	// Setup indexes
	err = repository.SetupIndexes(db)
	if err != nil {
		t.Fatalf("Failed to setup indexes: %v", err)
	}

	// Verify indexes
	collection := db.Collection("notes")
	cursor, err := collection.Indexes().List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list indexes: %v", err)
	}
	defer cursor.Close(context.Background())

	var indexes []bson.M
	if err = cursor.All(context.Background(), &indexes); err != nil {
		t.Fatalf("Failed to get indexes: %v", err)
	}

	// Expected indexes
	expectedIndexes := map[string]bool{
		"user_notes_date":         false,
		"user_id_index":           false,
		"user_pinned_notes_order": false,
		"text_search":             false,
		"user_tags":               false,
		"user_archived_notes":     false,
		"_id_":                    false, // Default index
	}

	// Check all indexes
	for _, index := range indexes {
		indexName := index["name"].(string)
		if _, exists := expectedIndexes[indexName]; exists {
			expectedIndexes[indexName] = true

			// Special check for text index
			if indexName == "text_search" {
				if weights, exists := index["weights"]; exists {
					weightsMap := weights.(bson.M)
					if weightsMap["title"].(int32) != 10 ||
						weightsMap["content"].(int32) != 5 ||
						weightsMap["tags"].(int32) != 3 {
						t.Error("Text index weights are not set correctly")
					}
				} else {
					t.Error("Text index weights not found")
				}
			}
		}
	}

	// Verify all expected indexes were found
	for name, found := range expectedIndexes {
		if !found && name != "_id_" { // Ignore the default _id index
			t.Errorf("Expected index %s was not created", name)
		}
	}

	// Cleanup
	if err := db.Collection("notes").Drop(context.Background()); err != nil {
		t.Errorf("Failed to cleanup test collection: %v", err)
	}
}
