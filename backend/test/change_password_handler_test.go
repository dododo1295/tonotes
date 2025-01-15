package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/handler"

	"main/model"
	"main/repository"
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

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,password"`
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
	oldPassword := "OldPass123!!"
	hashedOldPassword, _ := services.HashPassword(oldPassword)
	pastTime := time.Now().Add(-24 * time.Hour * 15) // 15 days ago

	testUser := model.User{
		UserID:             testUserID,
		Username:           "testuser",
		Email:              "test@example.com",
		Password:           hashedOldPassword,
		CreatedAt:          time.Now(),
		LastPasswordChange: pastTime,
	}

	// Set up test database
	collection := client.Database("tonotes_test").Collection("users")
	_, err = collection.InsertOne(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	defer collection.DeleteMany(context.Background(), bson.M{})

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		token, err := services.GenerateToken(testUserID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		c.Request.Header.Set("Authorization", "Bearer "+token)
		c.Set("user_id", testUserID)
		c.Set("users_repo", &repository.UsersRepo{MongoCollection: collection})
	})
	router.POST("/change-password", handler.ChangePasswordHandler)

	tests := []struct {
		name           string
		requestBody    map[string]string
		setupFunc      func()
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name: "Success - Valid password change",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "NewPass456!!",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]string{
				"message": "Password updated successfully",
			},
		},
		{
			name: "Failure - Same password",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": oldPassword,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "New password cannot be the same as current password",
			},
		},
		{
			name: "Failure - Rate limit",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "AnotherPass789!!",
			},
			setupFunc: func() {
				// Update LastPasswordChange to recent time
				_, err := collection.UpdateOne(
					context.Background(),
					bson.M{"user_id": testUserID},
					bson.M{"$set": bson.M{"lastPasswordChange": time.Now()}},
				)
				if err != nil {
					t.Fatalf("Failed to update LastPasswordChange: %v", err)
				}
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedBody: map[string]string{
				"error": "Password can only be changed every 2 weeks",
			},
		},
		{
			name: "Failure - Incorrect old password",
			requestBody: map[string]string{
				"old_password": "WrongPass123!!",
				"new_password": "NewPass789!!",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody: map[string]string{
				"error": "Current password is incorrect",
			},
		},
		{
			name: "Failure - Invalid new password format",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "weak",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]string{
				"error": "New password does not meet requirements",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset user state before each test
			_, err := collection.UpdateOne(
				context.Background(),
				bson.M{"user_id": testUserID},
				bson.M{"$set": bson.M{
					"password":           hashedOldPassword,
					"lastPasswordChange": pastTime,
				}},
			)
			if err != nil {
				t.Fatalf("Failed to reset user state: %v", err)
			}

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			// Convert request body to JSON
			jsonBody, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// Create request
			req, _ := http.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Parse response body
			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response body: %v", err)
			}

			// Check response message
			if tt.expectedBody != nil {
				for key, expectedValue := range tt.expectedBody {
					if actualValue, exists := response[key]; !exists || actualValue != expectedValue {
						t.Errorf("Expected response body to contain %s: %s, got %s", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}
