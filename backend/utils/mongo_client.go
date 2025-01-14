package utils

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient is a global variable holding the MongoDB client
var MongoClient *mongo.Client

// InitMongoClient initializes the MongoDB client from the environment variables
func InitMongoClient() {
	// Only try to load .env if not in test mode
	if os.Getenv("GO_ENV") != "test" {
		if err := godotenv.Load(); err != nil {
			log.Fatal("Error loading .env file")
		}
	}

	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MongoDB URI is not set")
	}

	// Initialize MongoDB client
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Assign the client to the global MongoClient variable
	MongoClient = client
}

func init() {
	// Only initialize if not in test mode
	if os.Getenv("GO_ENV") != "test" {
		InitMongoClient()
	}
}
