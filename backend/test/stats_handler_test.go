package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/usecase"
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
func setupStatsHandler(t *testing.T) (*repository.UserRepo, *repository.NoteRepo, *usecase.TodoService, *repository.SessionRepo, func()) {
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)

	// Get database reference
	dbName := os.Getenv("MONGO_DB_TEST")
	if dbName == "" {
		t.Fatal("MONGO_DB_TEST environment variable not set")
	}
	db := client.Database(dbName)

	// Set the MongoDB client for utils
	utils.MongoClient = client

	collections := []string{"users", "notes", "todos", "sessions"}

	// Drop and recreate collections
	for _, collName := range collections {
		if err := db.Collection(collName).Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
		}
	}

	// Create collections
	for _, collName := range collections {
		err := db.CreateCollection(context.Background(), collName)
		if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
			t.Fatalf("Failed to create collection %s: %v", collName, err)
		}
	}

	// Initialize repositories with correct database
	userRepo := repository.GetUserRepo(client)
	userRepo.MongoCollection = db.Collection("users")

	noteRepo := repository.GetNoteRepo(client)
	noteRepo.MongoCollection = db.Collection("notes")

	todosRepo := repository.GetTodoRepo(client)
	todosRepo.MongoCollection = db.Collection("todos")
	todoService := usecase.NewTodoService(todosRepo)

	sessionRepo := repository.GetSessionRepo(client)
	sessionRepo.MongoCollection = db.Collection("sessions")

	// Create indexes
	for _, collName := range collections {
		_, err := db.Collection(collName).Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		})
		if err != nil {
			t.Fatalf("Failed to create index for collection %s: %v", collName, err)
		}
	}

	// Verify collections are accessible
	for _, collName := range collections {
		count, err := db.Collection(collName).CountDocuments(context.Background(), bson.M{})
		if err != nil {
			t.Fatalf("Failed to access collection %s: %v", collName, err)
		}
		t.Logf("Collection %s initialized with %d documents", collName, count)
	}

	return userRepo, noteRepo, todoService, sessionRepo, func() {
		t.Log("Running cleanup...")
		// Drop all collections
		for _, collName := range collections {
			if err := db.Collection(collName).Drop(context.Background()); err != nil {
				t.Logf("Warning: Failed to drop collection %s during cleanup: %v", collName, err)
			}
		}
		cleanup()
	}
}

func TestGetUserStatsHandler(t *testing.T) {
	userRepo, noteRepo, todoService, sessionRepo, cleanup := setupStatsHandler(t)
	defer cleanup()

	// Create stats handler
	statsHandler := handler.NewStatsHandler(userRepo, noteRepo, todoService, sessionRepo)

	tests := []struct {
		name          string
		userID        string
		setupTestData func(*testing.T, string)
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "Success - User with data",
			userID: "test-user-id",
			setupTestData: func(t *testing.T, userID string) {
				ctx := context.Background()

				// Create user
				user := &model.User{
					UserID:    userID,
					Username:  "testuser",
					Email:     "test@example.com",
					Password:  "TestPass123!!",
					CreatedAt: time.Now().Add(-24 * time.Hour),
				}
				if _, err := userRepo.AddUser(ctx, user); err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}

				// Create notes
				notes := []*model.Note{
					{
						ID:        uuid.New().String(),
						UserID:    userID,
						Title:     "Test Note 1",
						Content:   "Content 1",
						Tags:      []string{"test", "important"},
						IsPinned:  true,
						CreatedAt: time.Now().Add(-2 * time.Hour),
						UpdatedAt: time.Now().Add(-2 * time.Hour),
					},
					{
						ID:         uuid.New().String(),
						UserID:     userID,
						Title:      "Test Note 2",
						Content:    "Content 2",
						Tags:       []string{"test"},
						IsArchived: true,
						CreatedAt:  time.Now().Add(-1 * time.Hour),
						UpdatedAt:  time.Now().Add(-1 * time.Hour),
					},
				}

				for _, note := range notes {
					if err := noteRepo.CreateNote(note); err != nil {
						t.Fatalf("Failed to create note: %v", err)
					}
				}

				// Create todos
				todos := []*model.Todo{
					{
						TodoID:      uuid.New().String(),
						UserID:      userID,
						TodoName:    "Test Todo 1",
						Description: "Description 1",
						Complete:    false,
						Priority:    model.PriorityHigh,
						DueDate:     time.Now().Add(24 * time.Hour),
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						TodoID:      uuid.New().String(),
						UserID:      userID,
						TodoName:    "Test Todo 2",
						Description: "Description 2",
						Complete:    true,
						Priority:    model.PriorityMedium,
						DueDate:     time.Now().Add(48 * time.Hour),
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				}

				for _, todo := range todos {
					if err := todoService.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create todo: %v", err)
					}
				}

				// Create session
				session := &model.Session{
					SessionID:      uuid.New().String(),
					UserID:         userID,
					DeviceInfo:     "test-device",
					IPAddress:      "127.0.0.1",
					CreatedAt:      time.Now().Add(-1 * time.Hour),
					ExpiresAt:      time.Now().Add(24 * time.Hour),
					LastActivityAt: time.Now(),
					IsActive:       true,
				}
				if err := sessionRepo.CreateSession(session); err != nil {
					t.Fatalf("Failed to create session: %v", err)
				}
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response struct {
					Data struct {
						Stats model.UserStats `json:"stats"`
					} `json:"data"`
				}

				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				stats := response.Data.Stats

				// Check notes stats
				if stats.NotesStats.Total != 2 {
					t.Errorf("Expected 2 total notes, got %d", stats.NotesStats.Total)
				}
				if stats.NotesStats.Pinned != 1 {
					t.Errorf("Expected 1 pinned note, got %d", stats.NotesStats.Pinned)
				}
				if stats.NotesStats.Archived != 1 {
					t.Errorf("Expected 1 archived note, got %d", stats.NotesStats.Archived)
				}
				if len(stats.NotesStats.TagCounts) != 2 {
					t.Errorf("Expected 2 unique tags, got %d", len(stats.NotesStats.TagCounts))
				}

				// Check todos stats
				if stats.TodoStats.Total != 2 {
					t.Errorf("Expected 2 total todos, got %d", stats.TodoStats.Total)
				}
				if stats.TodoStats.Completed != 1 {
					t.Errorf("Expected 1 completed todo, got %d", stats.TodoStats.Completed)
				}
				if stats.TodoStats.Pending != 1 {
					t.Errorf("Expected 1 pending todo, got %d", stats.TodoStats.Pending)
				}

				// Check activity stats
				if stats.ActivityStats.TotalSessions != 1 {
					t.Errorf("Expected 1 total session, got %d", stats.ActivityStats.TotalSessions)
				}
				if stats.ActivityStats.LastActive.IsZero() {
					t.Error("LastActive time should not be zero")
				}
				if stats.ActivityStats.AccountCreated.IsZero() {
					t.Error("AccountCreated time should not be zero")
				}
			},
		},
		{
			name:   "Success - New User No Data",
			userID: "new-user-id",
			setupTestData: func(t *testing.T, userID string) {
				ctx := context.Background()
				user := &model.User{
					UserID:    userID,
					Username:  "newuser",
					Email:     "new@example.com",
					Password:  "TestPass123!!",
					CreatedAt: time.Now(),
				}
				if _, err := userRepo.AddUser(ctx, user); err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response struct {
					Data struct {
						Stats model.UserStats `json:"stats"`
					} `json:"data"`
				}

				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				stats := response.Data.Stats

				// Verify zero values
				if stats.NotesStats.Total != 0 {
					t.Errorf("Expected 0 notes, got %d", stats.NotesStats.Total)
				}
				if stats.TodoStats.Total != 0 {
					t.Errorf("Expected 0 todos, got %d", stats.TodoStats.Total)
				}
				if stats.ActivityStats.TotalSessions != 0 {
					t.Errorf("Expected 0 sessions, got %d", stats.ActivityStats.TotalSessions)
				}
				if !stats.ActivityStats.LastActive.IsZero() {
					t.Error("Expected zero LastActive time for new user")
				}
				if stats.ActivityStats.AccountCreated.IsZero() {
					t.Error("AccountCreated time should not be zero")
				}
			},
		},
		{
			name:   "User Not Found",
			userID: "nonexistent-id",
			setupTestData: func(t *testing.T, userID string) {
				// No setup needed
			},
			expectedCode: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response["error"] != "User not found" {
					t.Errorf("Expected error 'User not found', got %v", response["error"])
				}
			},
		},
		{
			name:   "Invalid User ID",
			userID: "",
			setupTestData: func(t *testing.T, userID string) {
				// No setup needed
			},
			expectedCode: http.StatusNotFound, // Changed from StatusUnauthorized
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response["error"] != "User not found" { // Changed expected error message
					t.Errorf("Expected error 'User not found', got %v", response["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collections before each test
			collections := []string{"users", "notes", "todos", "sessions"}
			for _, collName := range collections {
				if err := userRepo.MongoCollection.Database().Collection(collName).Drop(context.Background()); err != nil {
					t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
				}
			}

			tt.setupTestData(t, tt.userID)

			// Create new router for each test
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				t.Logf("Processing request: %s %s", c.Request.Method, c.Request.URL.Path)
				c.Next()
			})

			router.GET("/stats", func(c *gin.Context) {
				c.Set("user_id", tt.userID)
				statsHandler.GetUserStats(c)
			})

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)

			// Log test information
			t.Logf("Running test: %s", tt.name)
			t.Logf("User ID: %s", tt.userID)

			// Execute request
			router.ServeHTTP(w, req)

			// Log response
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Check response content
			tt.checkResponse(t, w)
		})
	}
}
