package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/middleware"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	// Set test environment
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("JWT_SECRET_KEY", "test_secret")
	os.Setenv("PORT", "8080")
}

func cleanupTestDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Logf("Cleanup warning: %v", err)
		return
	}
	defer client.Disconnect(ctx)

	if err := client.Database("tonotes_test").Drop(ctx); err != nil {
		t.Logf("Cleanup warning: %v", err)
	}
}

// setupTestRouter creates a test router with minimal configuration
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(
		gin.Recovery(),
		middleware.RequestTracingMiddleware(),
		middleware.MetricsMiddleware(),
		middleware.RequestSizeLimiter(10<<20), // 10MB limit
	)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		health := map[string]interface{}{
			"status":   "up",
			"time":     time.Now(),
			"services": map[string]string{"mongodb": "up"},
		}
		c.JSON(http.StatusOK, health)
	})

	// Add the notes endpoint for testing
	router.POST("/api/notes", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	return router
}

func TestHealthCheck(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, ok := response["status"].(string); !ok || status != "up" {
		t.Errorf("Expected status 'up', got %v", status)
	}

	if _, exists := response["time"]; !exists {
		t.Error("Response missing time field")
	}

	if services, ok := response["services"].(map[string]interface{}); !ok {
		t.Error("Response missing services field")
	} else if _, exists := services["mongodb"]; !exists {
		t.Error("Services missing mongodb status")
	}
}

func TestRequestSizeLimiter(t *testing.T) {
	router := setupTestRouter()

	// Create a large request body (15MB)
	largeBody := make([]byte, 15<<20) // 15MB
	for i := range largeBody {
		largeBody[i] = 'A' // Fill with actual data
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/notes", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	// Explicitly set Content-Length header
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(largeBody)))

	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status code %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)

	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	if duration.Milliseconds() >= 1000 {
		t.Errorf("Request took too long: %v", duration)
	}
}

func TestServiceInitialization(t *testing.T) {
	// Test MongoDB initialization
	if err := utils.InitMongoClient(); err != nil {
		t.Errorf("Failed to initialize MongoDB client: %v", err)
	}

	if utils.MongoClient == nil {
		t.Error("MongoDB client is nil after initialization")
	}

	// Test connection
	if err := utils.CheckMongoConnection(); err != nil {
		t.Errorf("MongoDB connection check failed: %v", err)
	}

	// Test Redis initialization if URL is provided
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		// Test token blacklist
		blacklist, err := services.NewTokenBlacklist(redisURL)
		if err != nil {
			t.Errorf("Failed to create token blacklist: %v", err)
		} else {
			if blacklist == nil {
				t.Error("Token blacklist is nil after creation")
			}
			defer blacklist.Close()
		}

		// Test session cache
		sessionCache, err := services.NewSessionCache(redisURL)
		if err != nil {
			t.Errorf("Failed to create session cache: %v", err)
		} else {
			if sessionCache == nil {
				t.Error("Session cache is nil after creation")
			}
			defer sessionCache.Close()
		}
	}
}

func TestEnvironmentSetup(t *testing.T) {
	requiredVars := []string{
		"MONGO_URI",
		"MONGO_DB",
		"JWT_SECRET_KEY",
		"PORT",
	}

	for _, v := range requiredVars {
		if val := os.Getenv(v); val == "" {
			t.Errorf("Environment variable %s not set", v)
		}
	}
}

func TestMiddlewareChain(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)

	router.ServeHTTP(w, req)

	if requestID := w.Header().Get("X-Request-ID"); requestID == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestCleanup(t *testing.T) {
	// MongoDB cleanup
	if utils.MongoClient != nil {
		if err := utils.CloseMongoConnection(); err != nil {
			t.Errorf("Failed to close MongoDB connection: %v", err)
		}
	}

	// Redis cleanup with proper error handling
	if services.TokenBlacklist != nil && services.TokenBlacklist.IsConnected() {
		if err := services.TokenBlacklist.Close(); err != nil {
			t.Errorf("Failed to close token blacklist: %v", err)
		}
	}

	if services.GlobalSessionCache != nil && services.GlobalSessionCache.IsConnected() {
		if err := services.GlobalSessionCache.Close(); err != nil {
			t.Errorf("Failed to close session cache: %v", err)
		}
	}
}
func TestMain(m *testing.M) {
	// Setup
	gin.SetMode(gin.TestMode)
	utils.InitJWT()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestDB(&testing.T{})

	os.Exit(code)
}
