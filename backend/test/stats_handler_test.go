package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Setup function for stats testing
func setupStatsHandler(t *testing.T) (*gin.Engine, *mongo.Client, *repository.UserRepo, *repository.NotesRepo, *repository.TodosRepo, *repository.SessionRepo, func()) {
	// Set all required environment variables
	envVars := map[string]string{
		"MONGO_DB":           "tonotes_test",
		"USERS_COLLECTION":   "users",
		"NOTES_COLLECTION":   "notes",
		"TODOS_COLLECTION":   "todos",
		"SESSION_COLLECTION": "sessions",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}

	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	utils.MongoClient = client

	db := client.Database(os.Getenv("MONGO_DB"))
	collections := []string{"users", "notes", "todos", "sessions"}
	for _, collName := range collections {
		err := db.CreateCollection(context.Background(), collName)
		if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
			t.Fatalf("Failed to create collection %s: %v", collName, err)
		}
	}

	// Initialize repositories with correct collection references
	userRepo := &repository.UserRepo{
		MongoCollection: db.Collection("users"),
	}
	notesRepo := &repository.NotesRepo{
		MongoCollection: db.Collection("notes"),
	}
	todosRepo := &repository.TodosRepo{
		MongoCollection: db.Collection("todos"),
	}
	sessionRepo := &repository.SessionRepo{
		MongoCollection: db.Collection("sessions"),
	}

	// Initialize router with mock auth middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use mock middleware instead of real auth
	mockAuth := func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	}

	// Register route with mock auth
	statsHandler := handler.NewStatsHandler(userRepo, notesRepo, todosRepo, sessionRepo)
	router.GET("/stats", mockAuth, statsHandler.GetUserStats)

	// Ensure collections are properly initialized
	collections = []string{
		os.Getenv("USERS_COLLECTION"),
		os.Getenv("NOTES_COLLECTION"),
		os.Getenv("TODOS_COLLECTION"),
		os.Getenv("SESSION_COLLECTION"),
	}

	for _, collName := range collections {
		coll := db.Collection(collName)
		// Create index for userID if needed
		if _, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.D{{Key: "userId", Value: 1}},
		}); err != nil {
			t.Fatalf("Failed to create index for collection %s: %v", collName, err)
		}
	}

	return router, client, userRepo, notesRepo, todosRepo, sessionRepo, cleanup
}

func TestGetUserStatsHandler(t *testing.T) {
	router, _, userRepo, notesRepo, todosRepo, sessionRepo, cleanup := setupStatsHandler(t)
	defer cleanup()

	tests := []struct {
		name          string
		setupTestData func(t *testing.T, userID string)
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{

		{
			name: "Success - User with data",
			setupTestData: func(t *testing.T, userID string) {
				// Create user
				user := &model.User{
					UserID:    userID,
					Username:  "testuser",
					Password:  "TestPass123!!",
					Email:     "test@example.com",
					CreatedAt: time.Now().Add(-24 * time.Hour),
				}
				if _, err := userRepo.AddUser(context.Background(), user); err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}

				// Create notes
				notes := []*model.Notes{
					{
						ID:        uuid.New().String(),
						UserID:    userID,
						Title:     "Test Note 1",
						Content:   "Content 1",
						Tags:      []string{"work", "important"},
						CreatedAt: time.Now(),
					},
					{
						ID:        uuid.New().String(),
						UserID:    userID,
						Title:     "Test Note 2",
						Content:   "Content 2",
						IsPinned:  true,
						Tags:      []string{"personal"},
						CreatedAt: time.Now(),
					},
					{
						ID:         uuid.New().String(),
						UserID:     userID,
						Title:      "Test Note 3",
						Content:    "Content 3",
						IsArchived: true,
						CreatedAt:  time.Now(),
					},
				}

				for _, note := range notes {
					if err := notesRepo.CreateNote(note); err != nil {
						t.Fatalf("Failed to create test note: %v", err)
					}
				}

				// Create todos
				todos := []*model.Todos{
					{
						TodoID:   uuid.New().String(),
						UserID:   userID,
						TodoName: "Test Todo 1",
						Complete: true,
					},
					{
						TodoID:   uuid.New().String(),
						UserID:   userID,
						TodoName: "Test Todo 2",
						Complete: false,
					},
				}

				for _, todo := range todos {
					if err := todosRepo.CreateTodo(context.Background(), todo); err != nil {
						t.Fatalf("Failed to create test todo: %v", err)
					}
				}

				// Create session
				session := &model.Session{
					SessionID:      uuid.New().String(),
					UserID:         userID,
					CreatedAt:      time.Now().Add(-1 * time.Hour),
					ExpiresAt:      time.Now().Add(23 * time.Hour),
					LastActivityAt: time.Now(),
					IsActive:       true,
				}
				if err := sessionRepo.CreateSession(session); err != nil {
					t.Fatalf("Failed to create test session: %v", err)
				}
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// ... existing response checks ...
			},
		},
		{
			name: "User Not Found",
			setupTestData: func(t *testing.T, userID string) {
				// Intentionally empty - testing non-existent user
			},
			expectedCode: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "User not found" {
					t.Errorf("Expected 'User not found' error, got %v", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New().String()
			tt.setupTestData(t, userID)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)

			// Set test user ID in header instead of auth token
			req.Header.Set("X-User-ID", userID)

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d\nResponse body: %s",
					tt.expectedCode, w.Code, w.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}
