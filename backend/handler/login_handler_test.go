package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"main/model"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting GO_ENV=test in init")
	os.Setenv("GO_ENV", "test")

	// Set JWT-related environment variables BEFORE any JWT initialization happens
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")

	// Register custom password validator
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			password := fl.Field().String()
			// Implement your password validation rules here
			return len(password) >= 6

		})
	}
}

func TestLoginHandler(t *testing.T) {
	fmt.Println("Starting TestLoginHandler")
	t.Log("Test starting...")

	// Print current environment value
	fmt.Printf("GO_ENV value: %s\n", os.Getenv("GO_ENV"))

	// Set other environment variables
	t.Log("Setting other environment variables")
	envVars := map[string]string{
		"MONGO_URI":                     "mongodb://localhost:27017",
		"JWT_SECRET_KEY":                "test_secret",
		"JWT_EXPIRATION_TIME":           "3600",
		"REFRESH_TOKEN_EXPIRATION_TIME": "604800",
		"MONGO_DB":                      "tonotes_test",
		"USERS_COLLECTION":              "users",
	}

	for key, value := range envVars {
		t.Logf("Setting %s=%s", key, value)
		os.Setenv(key, value)
	}

	t.Log("Initializing MongoDB client...")
	testClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer testClient.Disconnect(context.Background())

	t.Log("Setting up MongoDB client...")
	utils.MongoClient = testClient

	// Clear the users collection before testing
	t.Log("Clearing test database...")
	if err := testClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}

	// Setup
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Use the function directly - remove the handler struct
	r.POST("/login", LoginHandler)

	// Test cases
	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		expectedError string
		setupMockDB   func(t *testing.T)
	}{
		{
			name: "Successful Login",
			inputJSON: `{
				"username": "testuser",
				"password": "Test12!!@@",
				"email": "test@example.com"
			}`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T) {
				// Setup test user in DB
				hashedPass, err := services.HashPassword("Test12!!@@")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}
				t.Logf("Original hashed password: %s", hashedPass)

				testUser := model.User{
					UserID:    "test-uuid",
					Username:  "testuser",
					Password:  hashedPass,
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}

				result, err := utils.MongoClient.Database("tonotes_test").Collection("users").InsertOne(context.Background(), testUser)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Inserted test user with ID: %v", result.InsertedID)

				// Immediately verify the stored password
				var storedUser model.User
				err = utils.MongoClient.Database("tonotes_test").Collection("users").
					FindOne(context.Background(), bson.M{"username": "testuser"}).
					Decode(&storedUser)
				if err != nil {
					t.Fatalf("Failed to retrieve test user: %v", err)
				}
				t.Logf("Stored password immediately after insert: %s", storedUser.Password)

				// Verify they match
				if hashedPass != storedUser.Password {
					t.Fatalf("Password mismatch after storage:\nOriginal:  %s\nStored:    %s",
						hashedPass, storedUser.Password)
				}

				// Test direct password verification
				match, err := services.VerifyPassword(storedUser.Password, "Test12!!@@")
				if err != nil {
					t.Fatalf("Password verification error: %v", err)
				}
				if !match {
					t.Fatal("Password verification failed immediately after insert")
				}
			},
		},
		{
			name: "Invalid Password",
			inputJSON: `{
				"username": "testuser",
				"password": "Test12!!##",
				"email": "test@example.com"
			}`,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Incorrect Password",
			setupMockDB: func(t *testing.T) {
				// Setup same test user but will try with different password
				hashedPass, err := services.HashPassword("Test12!!@@")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				testUser := model.User{
					UserID:    "test-uuid",
					Username:  "testuser",
					Password:  hashedPass,
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}

				_, err = utils.MongoClient.Database("tonotes_test").Collection("users").InsertOne(context.Background(), testUser)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
		},
		{
			name: "User Not Found",
			inputJSON: `{
				"username": "nonexistent",
				"password": "Test12!!@@",
				"email": "nonexistent@example.com"
			}`,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid username",
			setupMockDB:   func(t *testing.T) {}, // No setup needed for non-existent user
		},
		{
			name: "Invalid JSON",
			inputJSON: `{
				"username": "testuser",
				"password": 
			}`,
			expectedCode:  http.StatusBadRequest,
			expectedError: "Invalid Request",
			setupMockDB:   func(t *testing.T) {}, // No setup needed for invalid JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the database before each test
			if err := utils.MongoClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear test database: %v", err)
			}

			// Setup mock database state
			tt.setupMockDB(t)

			// Create request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			// Serve request
			r.ServeHTTP(w, req)

			// Log response for debugging
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Parse response
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check for error cases
			if tt.expectedError != "" {
				if errMsg, ok := response["error"].(string); !ok || errMsg != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errMsg)
				}
			} else {
				// Check successful login response
				if _, hasToken := response["token"]; !hasToken {
					t.Error("Response missing token")
				}
				if _, hasRefresh := response["refresh"]; !hasRefresh {
					t.Error("Response missing refresh token")
				}
				if msg, ok := response["message"].(string); !ok || msg != "Login successful" {
					t.Errorf("Expected message 'Login successful', got %q", msg)
				}
			}
		})
	}
}
