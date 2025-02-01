package testutils

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// SetupTestDB sets up a test database and returns a cleanup function
func SetupTestEnvironment() {
	envVars := map[string]string{
		"GO_ENV":              "test",
		"MONGO_URI":           "mongodb://localhost:27017",
		"MONGO_DB":            "tonotes_test",
		"USERS_COLLECTION":    "users",
		"NOTES_COLLECTION":    "notes",
		"TODOS_COLLECTION":    "todos",
		"JWT_SECRET_KEY":      "test_secret_key",
		"SESSIONS_COLLECTION": "sessions",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
		log.Printf("Set environment variable: %s=%s", key, value)
	}
}

// SetupTestDB sets up a test database and returns a cleanup function
func SetupTestDB(t *testing.T) (*mongo.Client, func()) {
	t.Log("Setting up test database")

	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Ping the database
	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
	t.Log("Connected to test database")

	// Create database
	db := client.Database(os.Getenv("MONGO_DB"))

	// Create collections
	collections := []string{"users", "sessions"}
	for _, collName := range collections {
		err := db.CreateCollection(context.Background(), collName)
		if err != nil {
			// Ignore NamespaceExists error
			if !strings.Contains(err.Error(), "NamespaceExists") {
				t.Logf("Warning creating collection %s: %v", collName, err)
			}
		}
		t.Logf("Ensured collection exists: %s", collName)
	}

	// Return cleanup function
	cleanup := func() {
		t.Log("Cleaning up test database")
		for _, collName := range collections {
			if err := db.Collection(collName).Drop(context.Background()); err != nil {
				t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
			}
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Logf("Warning: Failed to disconnect: %v", err)
		}
	}

	return client, cleanup
}
