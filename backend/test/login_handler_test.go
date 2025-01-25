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
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting GO_ENV=test in init")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("SESSION_COLLECTION", "sessions")

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			return len(fl.Field().String()) >= 6
		})
	}
}

// TestLoginHandler verifies the login functionality including:
// - User authentication with valid credentials
// - Session management and creation
// - Session limit enforcement (max 5 active sessions)
// - Least active session termination when session limit is reached
// - Response format and token generation
func TestLoginHandler(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	// Database cleanup
	if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear users collection: %v", err)
	}
	if err := client.Database("tonotes_test").Collection("sessions").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear sessions collection: %v", err)
	}

	sessionRepo := repository.GetSessionRepo(client)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/login", func(c *gin.Context) {
		handler.LoginHandler(c, sessionRepo)
	})

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UsersRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *repository.SessionRepo)
	}{
		{
			name: "Successful login - First session",
			inputJSON: `{
                "username": "test@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				user := &model.User{
					UserID:    "test-uuid",
					Username:  "test@example.com",
					Email:     "test@example.com",
					Password:  hashedPassword,
					CreatedAt: time.Now(),
				}

				if _, err := userRepo.MongoCollection.InsertOne(context.Background(), user); err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, ok := data["token"].(string); !ok {
					t.Error("Response missing token")
				}
				if _, ok := data["refresh"].(string); !ok {
					t.Error("Response missing refresh token")
				}
				if _, ok := data["notice"]; ok {
					t.Error("Notice should not be present for first login")
				}
			},
		},
		{
			name: "Login with max sessions - should end least active session",
			inputJSON: `{
                "username": "test@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				user := &model.User{
					UserID:    "test-uuid",
					Username:  "test@example.com",
					Email:     "test@example.com",
					Password:  hashedPassword,
					CreatedAt: time.Now(),
				}

				if _, err := userRepo.MongoCollection.InsertOne(context.Background(), user); err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}

				// Create max sessions with different activity times
				for i := 0; i < handler.MaxActiveSessions; i++ {
					session := &model.Session{
						SessionID:      fmt.Sprintf("session-%d", i),
						UserID:         "test-uuid",
						CreatedAt:      time.Now().Add(-time.Duration(i) * time.Hour),
						ExpiresAt:      time.Now().Add(24 * time.Hour),
						LastActivityAt: time.Now().Add(-time.Duration(i) * time.Hour),
						DeviceInfo:     fmt.Sprintf("device-%d", i),
						IPAddress:      "127.0.0.1",
						IsActive:       true,
					}
					if _, err := sessionRepo.MongoCollection.InsertOne(context.Background(), session); err != nil {
						t.Fatalf("Failed to insert test session: %v", err)
					}
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				// Check basic response fields
				if _, ok := data["token"].(string); !ok {
					t.Error("Response missing token")
				}
				if _, ok := data["refresh"].(string); !ok {
					t.Error("Response missing refresh token")
				}

				// Check notice about ended session
				notice, hasNotice := data["notice"].(string)
				if !hasNotice {
					t.Error("Response missing notice about ended session")
				} else if notice != "Logged out of least active session due to session limit" {
					t.Errorf("Unexpected notice message: %s", notice)
				}

				// Verify session state
				time.Sleep(100 * time.Millisecond) // Allow time for DB operations
				var sessions []*model.Session
				cursor, err := sessionRepo.MongoCollection.Find(context.Background(),
					bson.M{"user_id": "test-uuid"})
				if err != nil {
					t.Fatalf("Failed to query sessions: %v", err)
				}

				if err := cursor.All(context.Background(), &sessions); err != nil {
					t.Fatalf("Failed to decode sessions: %v", err)
				}

				// Count active sessions
				activeCount := 0
				var oldestInactiveID string
				oldestInactiveTime := time.Now()

				for _, session := range sessions {
					if session.IsActive {
						activeCount++
					} else {
						if session.LastActivityAt.Before(oldestInactiveTime) {
							oldestInactiveTime = session.LastActivityAt
							oldestInactiveID = session.SessionID
						}
					}
				}

				if activeCount != handler.MaxActiveSessions {
					t.Errorf("Expected %d active sessions, got %d",
						handler.MaxActiveSessions, activeCount)
				}

				if oldestInactiveID != fmt.Sprintf("session-%d", handler.MaxActiveSessions-1) {
					t.Errorf("Expected oldest session to be ended, got %s", oldestInactiveID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collections before each test
			if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear users collection: %v", err)
			}
			if err := client.Database("tonotes_test").Collection("sessions").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear sessions collection: %v", err)
			}

			userRepo := repository.GetUsersRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w, sessionRepo)
		})
	}
}
