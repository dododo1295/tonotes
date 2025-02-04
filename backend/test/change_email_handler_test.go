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
	"main/test/testutils"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	fmt.Println("Setting up change email test environment")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
	os.Setenv("MONGO_DB_TEST", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")
}

func TestChangeEmailHandler(t *testing.T) {
	// Setup test environment
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	utils.MongoClient = client

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		setupAuth     func() string
		requestBody   map[string]interface{}
		expectedCode  int
		setupTestData func(t *testing.T, userRepo *repository.UserRepo) string
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
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) string {
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
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) string {
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
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) string {
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
			setupTestData: func(t *testing.T, userRepo *repository.UserRepo) string {
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
			// Clear users collection before each test
			db := client.Database(os.Getenv("MONGO_DB"))
			if err := db.Collection("users").Drop(context.Background()); err != nil {
				t.Logf("Warning: Failed to drop users collection: %v", err)
			}

			// Initialize repository
			userRepo := repository.GetUserRepo(utils.MongoClient)

			// Setup test data and get userID
			userID := tt.setupTestData(t, userRepo)

			// Create request body
			jsonBody, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/change-email", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Set up authentication
			if token := tt.setupAuth(); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			// Set up router
			router := gin.New()
			router.POST("/change-email", func(c *gin.Context) {
				c.Set("user_id", userID)
				handler.ChangeEmailHandler(c)
			})

			// Log test information
			t.Logf("Test: %s", tt.name)
			t.Logf("Request Body: %s", jsonBody)

			// Execute request
			router.ServeHTTP(w, req)

			// Log response
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Run custom response checks
			tt.checkResponse(t, w)

			// Verify database state if needed
			if w.Code == http.StatusOK {
				updatedUser, err := userRepo.FindUser(userID)
				if err != nil {
					t.Fatalf("Failed to fetch updated user: %v", err)
				}

				expectedEmail := tt.requestBody["new_email"].(string)
				if updatedUser.Email != expectedEmail {
					t.Errorf("Expected user email to be %q, got %q", expectedEmail, updatedUser.Email)
				}

				// Verify LastEmailChange was updated
				if time.Since(updatedUser.LastEmailChange) > time.Second {
					t.Error("LastEmailChange was not updated properly")
				}
			}
		})
	}
}
