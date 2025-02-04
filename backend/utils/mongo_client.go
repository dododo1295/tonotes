package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoClient is a global variable holding the MongoDB client
// MongoClient is a global variable holding the MongoDB client
var (
	MongoClient *mongo.Client
	once        sync.Once
)

// MongoConfig holds the MongoDB connection configuration
type MongoConfig struct {
	URI             string
	MaxPoolSize     uint64
	MinPoolSize     uint64
	MaxConnIdleTime time.Duration
	RetryWrites     bool
	Database        string
	Username        string
	Password        string
}

// getMongoConfig loads MongoDB configuration from environment variables
func getMongoConfig() MongoConfig {
	// Load .env file if not in test environment
	if os.Getenv("GO_ENV") != "test" {
		if err := godotenv.Load(); err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	username := os.Getenv("MONGO_USERNAME")
	password := os.Getenv("MONGO_PASSWORD")

	// Build URI with authentication if credentials are provided
	uri := os.Getenv("MONGO_URI")
	if uri == "" && username != "" && password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@localhost:27017", username, password)
	}

	return MongoConfig{
		URI:             uri,
		MaxPoolSize:     GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100),
		MinPoolSize:     GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10),
		MaxConnIdleTime: time.Duration(GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)) * time.Second,
		RetryWrites:     GetEnvAsBool("MONGO_RETRY_WRITES", true),
		Database:        GetEnvAsString("MONGO_DB", "tonotes"),
		Username:        username,
		Password:        password,
	}
}

// InitMongoClient initializes the MongoDB client from the environment variables
func InitMongoClient() error {
	var err error
	once.Do(func() {
		err = initClient()
	})
	return err
}

// initClient handles the actual MongoDB client initialization
func initClient() error {
	if os.Getenv("GO_ENV") != "test" {
		if err := godotenv.Load(); err != nil {
			log.Fatal("Error loading .env file")
		}
	}

	config := getMongoConfig()
	if config.URI == "" {
		return fmt.Errorf("MongoDB URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetMaxConnIdleTime(config.MaxConnIdleTime).
		SetRetryWrites(config.RetryWrites)

	// Set auth credentials if available
	if config.Username != "" && config.Password != "" {
		clientOptions.SetAuth(options.Credential{
			Username: config.Username,
			Password: config.Password,
		})
	}

	clientOptions.SetPoolMonitor(&event.PoolMonitor{
		Event: func(evt *event.PoolEvent) {
			switch evt.Type {
			case event.GetSucceeded:
				log.Printf("Successfully got connection from pool")
			case event.ConnectionCreated:
				log.Printf("New connection created")
			case event.ConnectionClosed:
				log.Printf("Connection closed")
			case event.PoolCleared:
				log.Printf("Pool cleared")
			}
		},
	})

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	MongoClient = client
	log.Println("Successfully connected to MongoDB")
	return nil
}

// CheckMongoConnection verifies the MongoDB connection is healthy
func CheckMongoConnection() error {
	if MongoClient == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := MongoClient.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return nil
}

// CloseMongoConnection gracefully closes the MongoDB connection
func CloseMongoConnection() error {
	if MongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := MongoClient.Disconnect(ctx); err != nil {
			return fmt.Errorf("error disconnecting from MongoDB: %w", err)
		}
		log.Println("MongoDB connection closed")
	}
	return nil
}

func init() {
	if os.Getenv("GO_ENV") != "test" {
		if err := InitMongoClient(); err != nil {
			log.Fatal("Failed to initialize MongoDB client:", err)
		}
	}
}
