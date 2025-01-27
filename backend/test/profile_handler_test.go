package test

import (
	"context"
	"encoding/json"
	"fmt"
	"main/handler"
	"main/model"
	"main/repository"
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

func TestGetUserProfileHandler(t *testing.T) {
	fmt.Println("Starting TestGetUserProfileHandler")
	t.Log("Test starting...")

	envVars := map[string]string{
		"MONGO_URI":        "mongodb://localhost:27017",
		"MONGO_DB":         "tonotes_test",
		"USERS_COLLECTION": "users",
	}

	for key, value := range envVars {
		t.Logf("Setting %s=%s", key, value)
		os.Setenv(key, value)
	}

	testClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer testClient.Disconnect(context.Background())

	utils.MongoClient = testClient

	if err := testClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		userID        string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UserRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "Successful Profile Fetch",
			userID:       "test-uuid",
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
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

				if username, ok := profile["username"].(string); !ok || username != "testuser" {
					t.Errorf("Expected username 'testuser', got %v", username)
				}
				if email, ok := profile["email"].(string); !ok || email != "test@example.com" {
					t.Errorf("Expected email 'test@example.com', got %v", email)
				}
			},
		},
		{
			name:         "User Not Found",
			userID:       "nonexistent-uuid",
			expectedCode: http.StatusNotFound,
			setupMockDB:  func(t *testing.T, userRepo *repository.UserRepo) {},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := utils.MongoClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear test database: %v", err)
			}

			userRepo := repository.GetUserRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/profile", nil)

			// Create new router for each test case
			r := gin.Default()
			r.GET("/profile", func(c *gin.Context) {
				c.Set("user_id", tt.userID)
				handler.GetUserProfileHandler(c)
			})

			// Serve request
			r.ServeHTTP(w, req)

			// Log response for debugging
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Run custom response checks
			tt.checkResponse(t, w)
		})
	}
}
