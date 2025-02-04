package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	testutils.SetupTestEnvironment()
}

func setupProfileTest(t *testing.T) (*mongo.Client, func()) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)

	// Set up collections
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Create users collection
	err := db.CreateCollection(context.Background(), "users")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Logf("Warning: Failed to create users collection: %v", err)
	}

	return client, func() {
		t.Log("Running profile test cleanup")
		// Drop users collection
		if err := db.Collection("users").Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop users collection: %v", err)
		}
		cleanup()
	}
}

func TestGetUserProfileHandler(t *testing.T) {
	// Setup test environment
	client, cleanup := setupProfileTest(t)
	defer cleanup()

	// Set MongoDB client
	utils.MongoClient = client

	// Get database reference
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Setup Gin router
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		userID        string
		expectedCode  int
		setupTestData func(*testing.T, *repository.UserRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "Successful Profile Fetch",
			userID:       "test-uuid",
			expectedCode: http.StatusOK,
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) {
				testUser := model.User{
					UserID:    "test-uuid",
					Username:  "testuser",
					Password:  "hashedpassword",
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}

				_, err := userRepo.AddUser(context.Background(), &testUser)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created test user: %+v", testUser)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				profile, ok := data["profile"].(map[string]interface{})
				if !ok {
					t.Error("Response missing profile or wrong format")
					return
				}

				// Verify profile fields
				expectedFields := map[string]string{
					"username": "testuser",
					"email":    "test@example.com",
				}

				for field, expected := range expectedFields {
					if value, ok := profile[field].(string); !ok || value != expected {
						t.Errorf("Expected %s to be '%s', got '%v'", field, expected, value)
					}
				}
			},
		},
		{
			name:         "User Not Found",
			userID:       "nonexistent-uuid",
			expectedCode: http.StatusNotFound,
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) {
				// No setup needed for non-existent user test
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "User not found" {
					t.Errorf("Expected error 'User not found', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid User ID",
			userID:       "",
			expectedCode: http.StatusBadRequest,
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) {
				// No setup needed for invalid user ID test
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				expectedError := "Invalid user id" // Changed to match handler's response
				if response.Error != expectedError {
					t.Errorf("Expected error '%s', got %q", expectedError, response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear users collection before each test
			if err := db.Collection("users").Drop(context.Background()); err != nil {
				t.Logf("Warning: Failed to clear users collection: %v", err)
			}

			// Setup test data
			userRepo := repository.GetUserRepo(client)
			tt.setupTestData(t, userRepo)

			// Create new router for each test
			router := gin.New()
			router.Use(func(c *gin.Context) {
				// Add logging middleware
				t.Logf("Processing request: %s %s", c.Request.Method, c.Request.URL.Path)
				c.Next()
			})

			router.GET("/profile", func(c *gin.Context) {
				c.Set("user_id", tt.userID)
				handler.GetUserProfileHandler(c)
			})

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/profile", nil)

			// Log test information
			t.Logf("Running test: %s", tt.name)
			t.Logf("User ID: %s", tt.userID)

			// Execute request
			router.ServeHTTP(w, req)

			// Log response
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Check response content
			tt.checkResponse(t, w)
		})
	}
}
