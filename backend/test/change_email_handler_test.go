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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting up change email test environment")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")
}

func TestChangeEmailHandler(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		setupAuth     func() string
		requestBody   map[string]interface{}
		expectedCode  int
		setupTestData func(t *testing.T, userRepo *repository.UsersRepo) string
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Success - Valid email change",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			requestBody: map[string]interface{}{
				"new_email": "newemail@example.com",
			},
			expectedCode: http.StatusOK,
			setupTestData: func(t *testing.T, userRepo *repository.UsersRepo) string {
				userID := uuid.New().String()
				user := &model.User{
					UserID:          userID,
					Email:           "old@example.com",
					Username:        "testuser",
					Password:        "password",
					CreatedAt:       time.Now(),
					LastEmailChange: time.Now().Add(-15 * 24 * time.Hour), // 15 days ago
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
				return userID
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

				if msg, ok := data["message"].(string); !ok || msg != "Email updated successfully" {
					t.Errorf("Expected message 'Email updated successfully', got %q", msg)
				}

				if email, ok := data["email"].(string); !ok || email != "newemail@example.com" {
					t.Errorf("Expected email 'newemail@example.com', got %q", email)
				}
			},
		},
		{
			name: "Failure - Same email",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			requestBody: map[string]interface{}{
				"new_email": "old@example.com",
			},
			expectedCode: http.StatusBadRequest,
			setupTestData: func(t *testing.T, userRepo *repository.UsersRepo) string {
				userID := uuid.New().String()
				user := &model.User{
					UserID:          userID,
					Email:           "old@example.com",
					Username:        "testuser",
					Password:        "password",
					CreatedAt:       time.Now(),
					LastEmailChange: time.Now().Add(-15 * 24 * time.Hour),
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
				return userID
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "New email is same as current email" {
					t.Errorf("Expected error 'New email is same as current email', got %q", response.Error)
				}
			},
		},
		{
			name: "Failure - Rate limit",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			requestBody: map[string]interface{}{
				"new_email": "newemail@example.com",
			},
			expectedCode: http.StatusTooManyRequests,
			setupTestData: func(t *testing.T, userRepo *repository.UsersRepo) string {
				userID := uuid.New().String()
				user := &model.User{
					UserID:          userID,
					Email:           "old@example.com",
					Username:        "testuser",
					Password:        "password",
					CreatedAt:       time.Now(),
					LastEmailChange: time.Now(), // Recent change
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
				return userID
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Email can only be changed every 2 weeks" {
					t.Errorf("Expected error 'Email can only be changed every 2 weeks', got %q", response.Error)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok || data["next_allowed_change"] == nil {
					t.Error("Response missing next_allowed_change time")
				}
			},
		},
		{
			name: "Failure - Invalid email format",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			requestBody: map[string]interface{}{
				"new_email": "invalid-email",
			},
			expectedCode: http.StatusBadRequest,
			setupTestData: func(t *testing.T, userRepo *repository.UsersRepo) string {
				userID := uuid.New().String()
				user := &model.User{
					UserID:          userID,
					Email:           "old@example.com",
					Username:        "testuser",
					Password:        "password",
					CreatedAt:       time.Now(),
					LastEmailChange: time.Now().Add(-15 * 24 * time.Hour),
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
				return userID
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid email format" {
					t.Errorf("Expected error 'Invalid email format', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear users collection
			if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear users collection: %v", err)
			}

			userRepo := repository.GetUsersRepo(utils.MongoClient)
			userID := tt.setupTestData(t, userRepo)

			// Create request
			jsonBody, _ := json.Marshal(tt.requestBody)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/change-email", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Set up authentication
			if token := tt.setupAuth(); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			// Set up context
			router := gin.New()
			router.POST("/change-email", func(c *gin.Context) {
				c.Set("user_id", userID)
				handler.ChangeEmailHandler(c)
			})

			// Serve request
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Run custom response checks
			tt.checkResponse(t, w)
		})
	}
}
