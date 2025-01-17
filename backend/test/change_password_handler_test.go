package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/handler"

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

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,password"`
}

func TestChangePasswordHandler(t *testing.T) {
	// Setup test environment
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	testUserID := uuid.New().String()
	oldPassword := "OldPass123!!"
	hashedOldPassword, _ := services.HashPassword(oldPassword)
	pastTime := time.Now().Add(-24 * time.Hour * 15)

	testUser := model.User{
		UserID:             testUserID,
		Username:           "testuser",
		Email:              "test@example.com",
		Password:           hashedOldPassword,
		CreatedAt:          time.Now(),
		LastPasswordChange: pastTime,
	}

	collection := client.Database("tonotes_test").Collection("users")
	_, err = collection.InsertOne(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	defer collection.DeleteMany(context.Background(), bson.M{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		token, _ := services.GenerateToken(testUserID)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		c.Set("user_id", testUserID)
	})
	router.POST("/change-password", handler.ChangePasswordHandler)

	tests := []struct {
		name           string
		requestBody    map[string]string
		setupFunc      func()
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Success - Valid password change",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "NewPass456!!",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				data, ok := response.Data.(map[string]interface{})
				if !ok || data["message"] != "Password updated successfully" {
					t.Errorf("Expected success message, got %v", data)
				}
			},
		},
		{
			name: "Failure - Same password",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": oldPassword,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Error != "New password cannot be the same as current password" {
					t.Errorf("Expected same password error message, got %v", response.Error)
				}
			},
		},
		{
			name: "Failure - Rate limit",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "AnotherPass789!!",
			},
			setupFunc: func() {
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
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Error != "Password can only be changed every 2 weeks" {
					t.Errorf("Expected rate limit error message, got %v", response.Error)
				}
				data, ok := response.Data.(map[string]interface{})
				if !ok || data["next_allowed_change"] == nil {
					t.Error("Rate limit response missing next_allowed_change")
				}
			},
		},
		{
			name: "Failure - Incorrect old password",
			requestBody: map[string]string{
				"old_password": "WrongPass123!!",
				"new_password": "NewPass789!!",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Error != "Current password is incorrect" {
					t.Errorf("Expected incorrect password error message, got %v", response.Error)
				}
			},
		},
		{
			name: "Failure - Invalid new password format",
			requestBody: map[string]string{
				"old_password": oldPassword,
				"new_password": "weak",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Error != "New password does not meet requirements" {
					t.Errorf("Expected invalid password format error message, got %v", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset user state
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

			jsonBody, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			req, _ := http.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
