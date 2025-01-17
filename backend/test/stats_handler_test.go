package test

import (
	"context"
	"encoding/json"
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
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", "tonotes_test")
}

func TestGetUserStatsHandler(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client
	gin.SetMode(gin.TestMode)

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
				userRepo := repository.GetUsersRepo(client)
				user := &model.User{
					UserID:    userID,
					Username:  "testuser",
					Password:  "TestPass123!!", // Added password
					Email:     "test@example.com",
					CreatedAt: time.Now().Add(-24 * time.Hour),
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}

				// Create notes
				notesRepo := repository.GetNotesRepo(client)
				// Regular note
				note1 := &model.Notes{
					ID:        uuid.New().String(),
					UserID:    userID,
					Title:     "Test Note 1",
					Content:   "Content 1",
					Tags:      []string{"work", "important"},
					CreatedAt: time.Now(),
				}
				err = notesRepo.CreateNote(note1)
				if err != nil {
					t.Fatalf("Failed to create test note: %v", err)
				}

				// Pinned note
				note2 := &model.Notes{
					ID:        uuid.New().String(),
					UserID:    userID,
					Title:     "Test Note 2",
					Content:   "Content 2",
					IsPinned:  true,
					Tags:      []string{"personal"},
					CreatedAt: time.Now(),
				}
				err = notesRepo.CreateNote(note2)
				if err != nil {
					t.Fatalf("Failed to create test note: %v", err)
				}

				// Archived note
				note3 := &model.Notes{
					ID:         uuid.New().String(),
					UserID:     userID,
					Title:      "Test Note 3",
					Content:    "Content 3",
					IsArchived: true,
					CreatedAt:  time.Now(),
				}
				err = notesRepo.CreateNote(note3)
				if err != nil {
					t.Fatalf("Failed to create test note: %v", err)
				}

				// Create todos
				todosRepo := repository.GetTodosRepo(client)
				// Completed todo
				todo1 := &model.Todos{
					ID:       uuid.New().String(),
					UserID:   userID,
					TodoName: "Test Todo 1",
					Complete: true,
				}
				err = todosRepo.CreateTodo(todo1)
				if err != nil {
					t.Fatalf("Failed to create test todo: %v", err)
				}

				// Pending todo
				todo2 := &model.Todos{
					ID:       uuid.New().String(),
					UserID:   userID,
					TodoName: "Test Todo 2",
					Complete: false,
				}
				err = todosRepo.CreateTodo(todo2)
				if err != nil {
					t.Fatalf("Failed to create test todo: %v", err)
				}

				// Create session
				sessionRepo := repository.GetSessionRepo(client)
				session := &model.Session{
					SessionID:      uuid.New().String(),
					UserID:         userID,
					CreatedAt:      time.Now().Add(-1 * time.Hour),
					ExpiresAt:      time.Now().Add(23 * time.Hour),
					LastActivityAt: time.Now(),
					IsActive:       true,
				}
				err = sessionRepo.CreateSession(session)
				if err != nil {
					t.Fatalf("Failed to create test session: %v", err)
				}
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

				stats, ok := data["stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing stats object")
				}

				// Check notes stats
				notesStats, ok := stats["notes_stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing notes_stats")
				}
				if notesStats["total"].(float64) != 3 {
					t.Errorf("Expected 3 total notes, got %v", notesStats["total"])
				}
				if notesStats["pinned"].(float64) != 1 {
					t.Errorf("Expected 1 pinned note, got %v", notesStats["pinned"])
				}
				if notesStats["archived"].(float64) != 1 {
					t.Errorf("Expected 1 archived note, got %v", notesStats["archived"])
				}

				// Check todos stats
				todoStats, ok := stats["todo_stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing todo_stats")
				}
				if todoStats["total"].(float64) != 2 {
					t.Errorf("Expected 2 total todos, got %v", todoStats["total"])
				}
				if todoStats["completed"].(float64) != 1 {
					t.Errorf("Expected 1 completed todo, got %v", todoStats["completed"])
				}
				if todoStats["pending"].(float64) != 1 {
					t.Errorf("Expected 1 pending todo, got %v", todoStats["pending"])
				}

				// Check activity stats
				activityStats, ok := stats["activity_stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing activity_stats")
				}
				if activityStats["total_sessions"].(float64) != 1 {
					t.Errorf("Expected 1 total session, got %v", activityStats["total_sessions"])
				}

				// Verify last_active and account_created exist
				if _, exists := activityStats["last_active"]; !exists {
					t.Error("Missing last_active timestamp")
				}
				if _, exists := activityStats["account_created"]; !exists {
					t.Error("Missing account_created timestamp")
				}
			},
		},
		{
			name: "User Not Found",
			setupTestData: func(t *testing.T, userID string) {
				// Don't create any data
			},
			expectedCode: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
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
			// Clear collections before each test
			collections := []string{"users", "notes", "todos", "sessions"}
			for _, coll := range collections {
				if err := client.Database("tonotes_test").Collection(coll).Drop(context.Background()); err != nil {
					t.Fatalf("Failed to clear collection %s: %v", coll, err)
				}
			}

			userID := uuid.New().String()
			tt.setupTestData(t, userID)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)

			router := gin.New()
			router.GET("/stats", func(c *gin.Context) {
				c.Set("user_id", userID)
				handler.GetUserStatsHandler(c)
			})

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
