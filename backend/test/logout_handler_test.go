package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/model"
	"main/repository"
	"main/services"
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
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
}
func TestLogoutHandler(t *testing.T) {
	// Initialize MongoDB client
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Set up database
	db := client.Database("tonotes_test")
	sessionsCollection := db.Collection("sessions")
	sessionRepo := &repository.SessionRepo{
		MongoCollection: sessionsCollection,
	}

	// Setup Redis
	setupTestRedis(t)
	defer services.TokenBlacklist.Close()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		setupAuth     func() (string, string, *model.Session)
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *model.Session)
	}{
		{
			name: "Successful Logout",
			setupAuth: func() (string, string, *model.Session) {
				userID := "test-user-id"
				accessToken, _ := services.GenerateToken(userID)
				refreshToken, _ := services.GenerateRefreshToken(userID)

				session := &model.Session{
					SessionID:      uuid.New().String(),
					UserID:         userID,
					CreatedAt:      time.Now(),
					ExpiresAt:      time.Now().Add(24 * time.Hour),
					LastActivityAt: time.Now(),
					DeviceInfo:     "test device",
					IPAddress:      "127.0.0.1",
					IsActive:       true,
				}

				// Create session in database
				_, err := sessionsCollection.InsertOne(context.Background(), session)
				if err != nil {
					t.Fatalf("Failed to create session: %v", err)
				}

				return accessToken, refreshToken, session
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, session *model.Session) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if msg, ok := data["message"].(string); !ok || msg != "Successfully logged out" {
					t.Errorf("Expected message 'Successfully logged out', got %v", msg)
				}

				// Add delay to allow database operations to complete
				time.Sleep(100 * time.Millisecond)

				// Check session status in database
				var updatedSession model.Session
				err = sessionsCollection.FindOne(context.Background(),
					bson.M{"session_id": session.SessionID}).Decode(&updatedSession)
				if err != nil {
					t.Fatalf("Failed to find session: %v", err)
				}

				if updatedSession.IsActive {
					t.Error("Session should be marked as inactive")
				}
			},
		},
		{
			name: "Missing Token",
			setupAuth: func() (string, string, *model.Session) {
				return "", "", nil
			},
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, _ *model.Session) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if response["error"] != "Invalid access token" {
					t.Errorf("Expected error 'Invalid access token', got %v", response["error"])
				}
			},
		},
		{
			name: "Invalid Token Format",
			setupAuth: func() (string, string, *model.Session) {
				return "invalid-token", "", nil
			},
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, _ *model.Session) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if response["error"] != "Missing refresh token" {
					t.Errorf("Expected error 'Missing refresh token', got %v", response["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear sessions collection before each test
			sessionsCollection.Drop(context.Background())

			router := gin.New()
			accessToken, refreshToken, session := tt.setupAuth()

			router.POST("/logout", func(c *gin.Context) {
				if session != nil {
					c.Set("session", session)
				}
				handler.LogoutHandler(c, sessionRepo)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/logout", nil)

			if accessToken != "" {
				req.Header.Set("Authorization", "Bearer "+accessToken)
			}
			if refreshToken != "" {
				req.Header.Set("Refresh-Token", refreshToken)
			}

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w, session)
		})
	}
}
