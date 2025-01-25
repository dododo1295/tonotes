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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting up session handler test environment")
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("SESSION_COLLECTION", "sessions")
}

func TestSessionHandlers(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client
	sessionRepo := repository.GetSessionRepo(client)

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		endpoint      string
		method        string
		userID        string
		setupData     func(t *testing.T) string // returns sessionID if needed
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:     "Get Active Sessions - Success",
			endpoint: "/sessions",
			method:   http.MethodGet,
			userID:   "test-user-id",
			setupData: func(t *testing.T) string {
				session := &model.Session{
					SessionID:      uuid.New().String(),
					UserID:        "test-user-id",
					DisplayName:   "Test Session",
					DeviceInfo:    "Test Device",
					CreatedAt:     time.Now(),
					ExpiresAt:     time.Now().Add(24 * time.Hour),
					IsActive:      true,
				}
				err := sessionRepo.CreateSession(session)
				if err != nil {
					t.Fatalf("Failed to create test session: %v", err)
				}
				return session.SessionID
			},
			expectedCode: http.StatusOK,
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

				sessions, ok := data["sessions"].([]interface{})
				if !ok {
					t.Fatal("Response missing sessions array")
				}

				if len(sessions) != 1 {
					t.Errorf("Expected 1 session, got %d", len(sessions))
				}
			},
		},
		{
			name:     "Logout All Sessions - Success",
			endpoint: "/sessions/logout-all",
			method:   http.MethodPost,
			userID:   "test-user-id",
			setupData: func(t *testing.T) string {
				// Create multiple sessions
				for i := 0; i < 3; i++ {
					session := &model.Session{
						SessionID:    uuid.New().String(),
						UserID:      "test-user-id",
						DisplayName: fmt.Sprintf("Test Session %d", i),
						DeviceInfo:  "Test Device",
						CreatedAt:   time.Now(),
						ExpiresAt:   time.Now().Add(24 * time.Hour),
						IsActive:    true,
					}
					err := sessionRepo.CreateSession(session)
					if err != nil {
						t.Fatalf("Failed to create test session: %v", err)
					}
				}
				return ""
			},
			expectedCode: http.StatusOK,
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

				msg, ok := data["message"].(string)
				if !ok || msg != "Successfully logged out of all sessions" {
					t.Errorf("Expected success message, got %v", msg)
				}

				// Verify all sessions are ended
				activeSessions, _ := sessionRepo.GetUserActiveSessions("test-user-id")
				if len(activeSessions) != 0 {
					t.Errorf("Expected 0 active sessions, got %d", len(activeSessions))
				}
			},
		},
		{
			name:     "Logout Specific Session - Success",
			endpoint: "/sessions/{session_id}/logout",
			method:   http.MethodPost,
			userID:   "test-user-id",
			setupData: func(t *testing.T) string {
				session := &model.Session{
					SessionID:    uuid.New().String(),
					UserID:      "test-user-id",
					DisplayName: "Test Session",
					DeviceInfo:  "Test Device",
					CreatedAt:   time.Now(),
					ExpiresAt:   time.Now().Add(24 * time.Hour),
					IsActive:    true,
				}
				err := sessionRepo.CreateSession(session)
				if err != nil {
					t.Fatalf("Failed to create test session: %v", err)
				}
				return session.SessionID
			},
			expectedCode: http.StatusOK,
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

				msg, ok := data["message"].(string)
				if !ok || msg != "Successfully logged out of the session" {
					t.Errorf("Expected success message, got %v", msg)
				}
			},
		},
		// Add more test cases for other session handlers
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear sessions collection before each test
			if err := client.Database("tonotes_test").Collection("sessions").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear sessions collection: %v", err)
			}

			sessionID := ""
			if tt.setupData != nil {
				sessionID = tt.setupData(t)
			}

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", tt.userID)
				if sessionID != "" {
					c.Set("session_id", sessionID)
				}
			})

			// Register routes based on test case
			switch tt.endpoint {
			case "/sessions":
				router.GET("/sessions", func(c *gin.Context) {
					handler.GetActiveSessions(c, sessionRepo)
				})
			case "/sessions/logout-all":
				router.POST("/sessions/logout-all", func(c *gin.Context) {
					handler.LogoutAllSessions(c, sessionRepo)
				})
			case "/sessions/{session_id}/logout":
				router.POST("/sessions/:session_id/logout", func(c *gin.Context) {
					handler.LogoutSession(c, sessionRepo)
				})
			}

			// Create request
			req := httptest.NewRequest(tt.method, strings.Replace(tt.endpoint, "{session_id}", sessionID, 1), nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
