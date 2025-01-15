package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/handler"
	"main/middleware"
	"main/model"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting up change password test environment")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")
}

func TestChangePasswordHandler(t *testing.T) {
	// Additional environment setup
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")

	// Initialize MongoDB client
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	// Create test user
	testUserID := uuid.New().String()
	initialPassword := "Test12!!@@"
	hashedPassword, _ := services.HashPassword(initialPassword)

	testUser := model.User{
		UserID:    testUserID,
		Username:  "testuser",
		Email:     "test@example.com",
		Password:  hashedPassword,
		CreatedAt: time.Now(),
	}

	// Use environment variables for database and collection
	dbName := os.Getenv("MONGO_DB")
	collName := os.Getenv("USERS_COLLECTION")
	collection := utils.MongoClient.Database(dbName).Collection(collName)

	// Clear any existing data
	collection.Drop(context.Background())

	// Insert test user
	_, err = collection.InsertOne(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Verify user was inserted
	var foundUser model.User
	err = collection.FindOne(context.Background(), bson.M{"user_id": testUserID}).Decode(&foundUser)
	if err != nil {
		t.Fatalf("Failed to find inserted user: %v", err)
	}
	t.Logf("Found user in DB: %+v", foundUser)

	// Generate token
	token, err := services.GenerateToken(testUserID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	t.Logf("Test user ID: %s", testUserID)
	t.Logf("JWT_SECRET_KEY: %s", os.Getenv("JWT_SECRET_KEY"))
	t.Logf("Generated token: %s", token)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/change-password", middleware.AuthMiddleware(), handler.ChangePasswordHandler)

	tests := []struct {
		name          string
		inputJSON     string
		token         string
		expectedCode  int
		expectedError string
	}{
		{
			name:         "Successful Password Change",
			inputJSON:    `{"old_password":"Test12!!@@","new_password":"NewPass123!!"}`,
			token:        token,
			expectedCode: http.StatusOK,
		},
		{
			name:          "Invalid Old Password",
			inputJSON:     `{"old_password":"WrongPass123!!","new_password":"NewPass123!!"}`,
			token:         token,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid old password",
		},
		{
			name:          "No Auth Token",
			inputJSON:     `{"old_password":"OldPass123!!","new_password":"NewPass123!!"}`,
			token:         "",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid token",
		},
		{
			name:          "Invalid Password Format",
			inputJSON:     `{"old_password":"OldPass123!!","new_password":"weak"}`,
			token:         token,
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/change-password",
				bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			// Debug logging
			t.Logf("Test: %s", tt.name)
			t.Logf("Request Headers: %v", req.Header)
			t.Logf("Request Body: %s", tt.inputJSON)

			// Serve request
			router.ServeHTTP(w, req)

			// Debug logging
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Parse response
			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check error message if expected
			if tt.expectedError != "" {
				if errMsg, ok := response["error"].(string); !ok || errMsg != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errMsg)
				}
			} else {
				// Check success message
				if msg, ok := response["message"].(string); !ok || msg != "password updated successfully" {
					t.Errorf("Expected message 'password updated successfully', got %q", msg)
				}

				// Verify password was actually updated in database
				var updatedUser model.User
				err = collection.FindOne(context.Background(), bson.M{"user_id": testUserID}).Decode(&updatedUser)
				if err != nil {
					t.Fatalf("Failed to fetch updated user: %v", err)
				}

				// Verify new password works
				match, _ := services.VerifyPassword(updatedUser.Password, "NewPass123!!")
				if !match {
					t.Error("New password verification failed")
				}
			}
		})
	}

	// Cleanup
	collection.Drop(context.Background())
}
