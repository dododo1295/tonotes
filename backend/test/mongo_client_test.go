package test

import (
	"context"
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

func TestMongoConnection(t *testing.T) {
	// Set default environment variables for testing
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")

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
	os.Setenv("MONGO_DB", "tonotes_test")

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

func TestConnectionPooling(t *testing.T) {
	// Set test environment
	os.Setenv("MONGO_MAX_POOL_SIZE", "10")
	os.Setenv("MONGO_MIN_POOL_SIZE", "5")
	os.Setenv("MONGO_MAX_CONN_IDLE_TIME", "30")

	client, err := mongo.Connect(context.Background(), options.Client().
		ApplyURI("mongodb://localhost:27017").
		SetMaxPoolSize(10).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(30*time.Second))

	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Test concurrent connections
	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Perform a simple operation
			err := client.Database("test").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
			if err != nil {
				t.Errorf("Connection %d failed: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify metrics
	metrics := utils.GetMongoMetrics()
	if metrics.ActiveConnections > 10 {
		t.Errorf("Pool size exceeded maximum: got %d, want <= 10", metrics.ActiveConnections)
	}
}

func TestConnectionMonitoring(t *testing.T) {
	// Initialize metrics
	metrics := &utils.MongoMetrics{
		LastCheckTime: time.Now(),
	}

	// Create monitored client
	clientOpts := options.Client().
		ApplyURI("mongodb://localhost:27017").
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

	// Perform operations to trigger events
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		// Fix unkeyed bson.D
		err := client.Database("test").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
		if err != nil {
			t.Errorf("Operation %d failed: %v", i, err)
		}
	}

	// Verify metrics
	if metrics.CreatedConnections == 0 {
		t.Error("No connections were created")
	}

	if metrics.ClosedConnections > metrics.CreatedConnections {
		t.Error("More connections closed than created")
	}
}

func TestConnectionRecovery(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().
		ApplyURI("mongodb://localhost:27017").
		SetServerSelectionTimeout(2*time.Second))

	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Test reconnection after failure
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

			// Check if error is retryable
			if cmdErr, ok := err.(mongo.CommandError); ok {
				isRetryable := cmdErr.Labels != nil && len(cmdErr.Labels) > 0
				if isRetryable != tt.shouldRetry {
					t.Errorf("Expected retryable=%v, got %v", tt.shouldRetry, isRetryable)
				}
			}
		})
	}
}
