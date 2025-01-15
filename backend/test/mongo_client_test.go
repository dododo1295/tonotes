package test

import (
	"context"
	"main/utils"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoConnection(t *testing.T) {
	// Set default environment variables for testing
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "Successful Connection",
			setup: func() {
				// Use valid local MongoDB URI
				os.Setenv("MONGO_URI", "mongodb://localhost:27017")
			},
			wantErr: false,
		},
		{
			name: "Invalid MongoDB URI",
			setup: func() {
				// Use a host that doesn't exist - this will cause a connection timeout
				os.Setenv("MONGO_URI", "mongodb://nonexistent-host:27017")
			},
			wantErr: true,
		},
		{
			name: "Empty MongoDB URI",
			setup: func() {
				os.Setenv("MONGO_URI", "")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup for this specific test case
			tt.setup()

			// Create a context with 2-second timeout to avoid long waits
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Try to connect to MongoDB
			client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))

			// If we expect an error
			if tt.wantErr {
				if err == nil {
					// Try to ping to force a connection attempt
					err = client.Ping(ctx, nil)
				}
				if err == nil {
					t.Error("Expected an error but got none")
				}
				return
			}

			// If we don't expect an error but got one
			if err != nil {
				t.Errorf("Failed to connect to MongoDB: %v", err)
				return
			}

			// Verify we can ping the database
			err = client.Ping(ctx, nil)
			if err != nil {
				t.Errorf("Failed to ping MongoDB: %v", err)
				return
			}

			// Test database selection
			dbName := "tonotes_test"
			db := client.Database(dbName)
			if db == nil {
				t.Error("Failed to get database reference")
				return
			}

			// Clean up: disconnect from MongoDB
			defer func() {
				if err = client.Disconnect(ctx); err != nil {
					t.Errorf("Failed to disconnect: %v", err)
				}
			}()

			// Verify the global MongoDB client is set
			if utils.MongoClient == nil {
				t.Error("MongoClient is nil")
			}
		})
	}
}
func TestMongoOperations(t *testing.T) {
	// Set environment variables
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")

	// Setup MongoDB connection
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Test database operations
	db := client.Database("tonotes_test")
	collection := db.Collection("test_collection")

	// Clear collection before testing
	if err := collection.Drop(ctx); err != nil {
		t.Logf("Warning: Failed to clear collection: %v", err)
	}

	// Test insert
	_, err = collection.InsertOne(ctx, bson.M{"test": "data"})
	if err != nil {
		t.Errorf("Failed to insert document: %v", err)
	}

	// Test find
	var result bson.M
	err = collection.FindOne(ctx, bson.M{"test": "data"}).Decode(&result)
	if err != nil {
		t.Errorf("Failed to find document: %v", err)
	}

	// Verify result
	if result["test"] != "data" {
		t.Errorf("Expected test field to be 'data', got %v", result["test"])
	}
}
