package test

import (
	"context"
	"main/test/testutils"
	"main/utils"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	testutils.SetupTestEnvironment()
}

func TestMongoConnection(t *testing.T) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "Successful Connection",
			setup: func() {
				// Using TEST_MONGO_URI from environment
				t.Logf("Using MongoDB URI: %s", os.Getenv("TEST_MONGO_URI"))
			},
			wantErr: false,
		},
		{
			name: "Invalid MongoDB URI",
			setup: func() {
				// Temporarily override the URI
				os.Setenv("TEST_MONGO_URI", "mongodb://nonexistent-host:27017")
			},
			wantErr: true,
		},
		{
			name: "Empty MongoDB URI",
			setup: func() {
				os.Setenv("TEST_MONGO_URI", "")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup for this specific test case
			tt.setup()

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Get MongoDB options from environment
			opts := options.Client().
				ApplyURI(os.Getenv("TEST_MONGO_URI")).
				SetMaxPoolSize(utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100)).
				SetMinPoolSize(utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10)).
				SetMaxConnIdleTime(time.Duration(utils.GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)) * time.Second)

			// Try to connect to MongoDB
			client, err := mongo.Connect(ctx, opts)

			// Reset environment after test
			defer testutils.SetupTestEnvironment()

			if tt.wantErr {
				if err == nil {
					err = client.Ping(ctx, nil)
				}
				if err == nil {
					t.Error("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to connect to MongoDB: %v", err)
				return
			}

			// Verify connection
			err = client.Ping(ctx, nil)
			if err != nil {
				t.Errorf("Failed to ping MongoDB: %v", err)
				return
			}

			// Test database selection
			db := client.Database(os.Getenv("MONGO_DB_TEST"))
			if db == nil {
				t.Error("Failed to get database reference")
				return
			}

			// Clean up
			defer func() {
				if err = client.Disconnect(ctx); err != nil {
					t.Errorf("Failed to disconnect: %v", err)
				}
			}()
		})
	}
}

func TestMongoOperations(t *testing.T) {
	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	db := client.Database(os.Getenv("MONGO_DB_TEST"))
	collection := db.Collection("test_collection")

	// Clear collection before testing
	if err := collection.Drop(ctx); err != nil {
		t.Logf("Warning: Failed to clear collection: %v", err)
	}

	// Test insert
	_, err := collection.InsertOne(ctx, bson.M{"test": "data"})
	if err != nil {
		t.Errorf("Failed to insert document: %v", err)
	}

	// Test find
	var result bson.M
	err = collection.FindOne(ctx, bson.M{"test": "data"}).Decode(&result)
	if err != nil {
		t.Errorf("Failed to find document: %v", err)
	}

	if result["test"] != "data" {
		t.Errorf("Expected test field to be 'data', got %v", result["test"])
	}
}

func TestConnectionPooling(t *testing.T) {
	// Get pool settings from environment
	maxPoolSize := utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100)
	minPoolSize := utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10)
	maxConnIdleTime := utils.GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)

	client, err := mongo.Connect(context.Background(), options.Client().
		ApplyURI(os.Getenv("TEST_MONGO_URI")).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetMaxConnIdleTime(time.Duration(maxConnIdleTime)*time.Second))

	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Test concurrent connections
	var wg sync.WaitGroup
	for i := 0; i < int(maxPoolSize+5); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := client.Database("test").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
			if err != nil {
				t.Errorf("Connection %d failed: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify metrics
	metrics := utils.GetMongoMetrics()
	if metrics.ActiveConnections > int64(maxPoolSize) {
		t.Errorf("Pool size exceeded maximum: got %d, want <= %d",
			metrics.ActiveConnections, maxPoolSize)
	}
}

func TestConnectionMonitoring(t *testing.T) {
	metrics := &utils.MongoMetrics{
		LastCheckTime: time.Now(),
	}

	clientOpts := options.Client().
		ApplyURI(os.Getenv("TEST_MONGO_URI")).
		SetPoolMonitor(&event.PoolMonitor{
			Event: func(evt *event.PoolEvent) {
				switch evt.Type {
				case event.ConnectionCreated:
					atomic.AddInt64(&metrics.CreatedConnections, 1)
				case event.ConnectionClosed:
					atomic.AddInt64(&metrics.ClosedConnections, 1)
				}
			},
		})

	client, err := mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := client.Database("test").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
		if err != nil {
			t.Errorf("Operation %d failed: %v", i, err)
		}
	}

	if metrics.CreatedConnections == 0 {
		t.Error("No connections were created")
	}

	if metrics.ClosedConnections > metrics.CreatedConnections {
		t.Error("More connections closed than created")
	}
}

func TestConnectionRecovery(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().
		ApplyURI(os.Getenv("TEST_MONGO_URI")).
		SetServerSelectionTimeout(2*time.Second))

	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	tests := []struct {
		name        string
		operation   func() error
		shouldRetry bool
	}{
		{
			name: "Temporary Network Failure",
			operation: func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				return client.Database("test").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
			},
			shouldRetry: true,
		},
		{
			name: "Authentication Failure",
			operation: func() error {
				ctx := context.Background()
				return client.Database("admin").RunCommand(ctx, bson.D{
					{Key: "auth", Value: bson.D{
						{Key: "user", Value: "invalid"},
						{Key: "pwd", Value: "invalid"},
					}},
				}).Err()
			},
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if err == nil {
				return
			}

			if cmdErr, ok := err.(mongo.CommandError); ok {
				isRetryable := cmdErr.Labels != nil && len(cmdErr.Labels) > 0
				if isRetryable != tt.shouldRetry {
					t.Errorf("Expected retryable=%v, got %v", tt.shouldRetry, isRetryable)
				}
			}
		})
	}
}
