package test

import (
	"context"
	"main/test/testutils"
	"main/utils"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestBasicDBConnection(t *testing.T) {
	// Setup test environment
	testutils.SetupTestEnvironment()

	// Get MongoDB connection settings from environment
	uri := os.Getenv("TEST_MONGO_URI")
	username := os.Getenv("MONGO_USERNAME")
	password := os.Getenv("MONGO_PASSWORD")
	dbName := os.Getenv("MONGO_DB_TEST")

	// Configure connection options with pooling
	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100)).
		SetMinPoolSize(utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10)).
		SetMaxConnIdleTime(time.Duration(utils.GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)) * time.Second)

	// Add authentication if credentials are provided
	if username != "" && password != "" {
		opts.SetAuth(options.Credential{
			Username: username,
			Password: password,
		})
	}

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Warning: Failed to disconnect: %v", err)
		}
	}()

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}

	// List available databases
	dbs, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		t.Fatalf("Failed to list databases: %v", err)
	}
	t.Logf("Available databases: %v", dbs)

	// Create test database and collection
	db := client.Database(dbName)
	testCollName := "test_collection"

	// Drop collection if it exists
	if err := db.Collection(testCollName).Drop(ctx); err != nil {
		t.Logf("Warning: Failed to drop existing collection: %v", err)
	}

	// Create collection with options
	createOpts := options.CreateCollection().
		SetCapped(false)

	err = db.CreateCollection(ctx, testCollName, createOpts)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	// Verify collection creation
	colls, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}
	t.Logf("Collections in %s: %v", dbName, colls)

	// Test connection pool settings
	t.Run("Connection Pool Settings", func(t *testing.T) {
		// Create event monitor for pool
		monitor := &event.PoolMonitor{
			Event: func(evt *event.PoolEvent) {
				switch evt.Type {
				case event.GetSucceeded:
					t.Logf("Connection acquired from pool")
				case event.ConnectionCreated:
					t.Logf("New connection created")
				case event.ConnectionClosed:
					t.Logf("Connection closed")
				}
			},
		}

		// Create new client with monitoring
		monitorOpts := options.Client().
			SetPoolMonitor(monitor).
			SetMaxPoolSize(utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100)).
			SetMinPoolSize(utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10))

		monitorClient, err := mongo.Connect(ctx, monitorOpts)
		if err != nil {
			t.Fatalf("Failed to create monitored client: %v", err)
		}
		defer monitorClient.Disconnect(ctx)

		// Perform some operations to test pool
		for i := 0; i < 5; i++ {
			if err := monitorClient.Ping(ctx, nil); err != nil {
				t.Errorf("Ping %d failed: %v", i, err)
			}
		}
	})

	// Clean up
	if err := db.Collection(testCollName).Drop(ctx); err != nil {
		t.Logf("Warning: Failed to clean up test collection: %v", err)
	}
}
