package test

import (
	"context"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestBasicDBConnection(t *testing.T) {
	// 1. Set environment variables
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", "tonotes_test")

	// 2. Create basic connection
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// 3. Print available databases
	dbs, err := client.ListDatabaseNames(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to list databases: %v", err)
	}
	t.Logf("Available databases: %v", dbs)

	// 4. Create test database and collection
	db := client.Database("tonotes_test")
	err = db.CreateCollection(context.Background(), "test_collection")
	if err != nil {
		t.Logf("Collection creation error (might already exist): %v", err)
	}

	// 5. List collections
	colls, err := db.ListCollectionNames(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}
	t.Logf("Collections in tonotes_test: %v", colls)
}
