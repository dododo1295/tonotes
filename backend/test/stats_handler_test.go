package test

import (
	"context"
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
func setupStatsHandler(t *testing.T) (*gin.Engine, *repository.UserRepo, *repository.NotesRepo, *usecase.TodosService, *repository.SessionRepo, func()) {
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
		if err := db.Collection(collName).Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
		}
	}

	for _, collName := range collections {
		err := db.CreateCollection(context.Background(), collName)
		if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
			t.Fatalf("Failed to create collection %s: %v", collName, err)
		}
	}

	for _, collName := range collections {
		count, err := db.Collection(collName).CountDocuments(context.Background(), bson.M{})
		if err != nil {
			t.Logf("Error checking collection %s: %v", collName, err)
		} else {
			t.Logf("Collection %s exists with %d documents", collName, count)
		}
	}

	userRepo := &repository.UserRepo{
		MongoCollection: db.Collection("users"),
	}
	notesRepo := &repository.NotesRepo{
		MongoCollection: db.Collection("notes"),
	}
	todosService := usecase.NewTodosService(repository.GetTodosRepo(client))
	sessionRepo := &repository.SessionRepo{
		MongoCollection: db.Collection("sessions"),
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockAuth := func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	}

	statsHandler := handler.NewStatsHandler(userRepo, notesRepo, todosService, sessionRepo)
	router.GET("/stats", mockAuth, statsHandler.GetUserStats)

	collections = []string{
		os.Getenv("USERS_COLLECTION"),
		os.Getenv("NOTES_COLLECTION"),
		os.Getenv("TODOS_COLLECTION"),
		os.Getenv("SESSION_COLLECTION"),
	}

	for _, collName := range collections {
		coll := db.Collection(collName)
		if _, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		}); err != nil {
			t.Fatalf("Failed to create index for collection %s: %v", collName, err)
		}
	}

	return router, userRepo, notesRepo, todosService, sessionRepo, cleanup
}

func TestGetUserStatsHandler(t *testing.T) {
	router, userRepo, notesRepo, todosService, sessionRepo, cleanup := setupStatsHandler(t)
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
				t.Log("User created successfully")

				// Verify user creation
				createdUser, err := userRepo.FindUser(userID)
				if err != nil || createdUser == nil {
					t.Fatalf("Failed to verify user creation: %v", err)
				}
				t.Log("User verified successfully")

				// Create notes
				notes := []*model.Notes{
					// ... your existing notes setup ...
				}

				for i, note := range notes {
					if err := notesRepo.CreateNote(note); err != nil {
						t.Fatalf("Failed to create note %d: %v", i+1, err)
					}
				}

				// Verify notes creation
				createdNotes, err := notesRepo.GetUserNotes(userID)
				if err != nil {
					t.Fatalf("Failed to verify notes: %v", err)
				}
				t.Logf("Created %d notes successfully", len(createdNotes))

				// Create todos
				todos := []*model.Todos{
					// ... your existing todos setup ...
				}

				for i, todo := range todos {
					if err := todosService.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create todo %d: %v", i+1, err)
					}
				}

				// Verify todos creation
				createdTodos, err := todosService.CountUserTodos(ctx, userID)
				if err != nil {
					t.Fatalf("Failed to verify todos: %v", err)
				}
				t.Logf("Created %d todos successfully", createdTodos)

				// Create session
				session := &model.Session{
					// ... your existing session setup ...
				}
				if err := sessionRepo.CreateSession(session); err != nil {
					t.Fatalf("Failed to create session: %v", err)
				}

				// Verify session creation
				sessions, err := sessionRepo.GetUserActiveSessions(userID)
				if err != nil {
					t.Fatalf("Failed to verify session: %v", err)
				}
				t.Logf("Created %d sessions successfully", len(sessions))

				// Final verification of all data
				t.Log("=== Final Data Verification ===")
				t.Logf("User ID: %s", userID)
				t.Logf("Notes count: %d", len(createdNotes))
				t.Logf("Todos count: %d", createdTodos)
				t.Logf("Sessions count: %d", len(sessions))
			},
			expectedCode: http.StatusOK,
			// ... rest of your test case remains the same ...
		},
		// ... your "User Not Found" test case remains the same ...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New().String()
			t.Logf("Testing with user ID: %s", userID)

			tt.setupTestData(t, userID)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)
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
