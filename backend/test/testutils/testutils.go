package testutils

import (
	"context"
	"log"
	"main/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MockableTime is an interface for mocking time.Now() in tests
type MockableTime interface {
	Now() time.Time
}

// RealTime implements MockableTime using time.Now()
type RealTime struct{}

func (RealTime) Now() time.Time {
	return time.Now()
}

// FixedTime implements MockableTime using a fixed time
type FixedTime struct {
	Fixed time.Time
}

func (ft FixedTime) Now() time.Time {
	return ft.Fixed
}

// Helper function to set the mock time for tests
func SetMockTime(mockTime MockableTime) {
	mockTimeGlobal = mockTime
}

var mockTimeGlobal MockableTime = RealTime{} // Global variable to hold the time implementation

// Add a mutex to protect environment variable access
var envMutex sync.Mutex

// SetupTestEnvironment sets up the test environment variables
func SetupTestEnvironment() {
	// Find and load the main .env file
	rootDir := findProjectRoot()
	if envPath := filepath.Join(rootDir, ".env"); rootDir != "" {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Warning: Could not load .env file: %v", err)
		} else {
			log.Printf("Loaded .env file from: %s", envPath)
		}
	}

	// Set test environment variables
	envMutex.Lock()                               // Acquire the lock
	defer envMutex.Unlock()                         // Release the lock when the function exits

	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_USERNAME", "admin")
	os.Setenv("MONGO_PASSWORD", "mongodblmpvBMCqJ3Ig2eX2oCTlNbf7TJ5533L80TvM8LC")

	// JWT secret key
	utils.JWTSecretKey = os.Getenv("JWT_SECRET_KEY")
	if utils.JWTSecretKey == "" {
		log.Fatal("JWT_SECRET_KEY environment variable not set")
	}
	// Construct MongoDB URI with credentials
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Replace template variables in URI
	mongoURI = strings.Replace(mongoURI, "${MONGO_USERNAME}", os.Getenv("MONGO_USERNAME"), -1)
	mongoURI = strings.Replace(mongoURI, "${MONGO_PASSWORD}", os.Getenv("MONGO_PASSWORD"), -1)

	os.Setenv("TEST_MONGO_URI", mongoURI)
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("MONGO_DB_TEST", "tonotes_test")

	// Set connection pool settings
	os.Setenv("MONGO_MAX_POOL_SIZE", "100")
	os.Setenv("MONGO_MIN_POOL_SIZE", "10")
	os.Setenv("MONGO_MAX_CONN_IDLE_TIME", "60")
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// SetupTestDB sets up a test database and returns a cleanup function
func SetupTestDB(t *testing.T) (*mongo.Client, func()) {
	// Ensure test environment is set up
	if os.Getenv("GO_ENV") != "test" {
		SetupTestEnvironment()
	}

	// Get MongoDB URI and credentials
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Fatal("TEST_MONGO_URI environment variable not set")
	}

	// Configure MongoDB client options with connection pooling
	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(utils.GetEnvAsUint64("MONGO_MAX_POOL_SIZE", 100)).
		SetMinPoolSize(utils.GetEnvAsUint64("MONGO_MIN_POOL_SIZE", 10)).
		SetMaxConnIdleTime(time.Duration(utils.GetEnvAsInt("MONGO_MAX_CONN_IDLE_TIME", 60)) * time.Second)

	// Add credentials if provided
	if username := os.Getenv("MONGO_USERNAME"); username != "" {
		opts.SetAuth(options.Credential{
			Username: username,
			Password: os.Getenv("MONGO_PASSWORD"),
		})
	}

	// Connect to MongoDB with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Verify connection
	if err = client.Ping(ctx, nil); err != nil {
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}

	// Setup cleanup function
	cleanup := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Drop test database
		dbName := os.Getenv("MONGO_DB_TEST") // Get the test database name

		if dbName != "" {
			if err := client.Database(dbName).Drop(ctx); err != nil {
				t.Logf("Warning: Failed to drop test database %s: %v", dbName, err)
			}
		}

		// Disconnect client
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Warning: Failed to disconnect: %v", err)
		}
	}

	return client, cleanup
}

// Helper function for tests to verify environment
func VerifyTestEnvironment(t *testing.T) {
	requiredVars := []string{
		"MONGO_USERNAME",
		"MONGO_PASSWORD",
		"MONGO_URI",
		"MONGO_DB",
		"MONGO_DB_TEST",
		"TEST_MONGO_URI",
	}

	t.Log("Verifying test environment configuration:")
	for _, v := range requiredVars {
		value := os.Getenv(v)
		if value == "" {
			t.Errorf("Required environment variable %s is missing", v)
		} else {
			t.Logf("%s is set", v)
		}
	}

	// Verify we're using test database
	if os.Getenv("MONGO_DB") != os.Getenv("MONGO_DB_TEST") {
		t.Error("Test environment is not using test database")
	}
}
