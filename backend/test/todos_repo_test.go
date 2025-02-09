package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	testutils.SetupTestEnvironment()

	// Ensure all required environment variables are set
	requiredVars := map[string]string{
		"MONGO_USERNAME": "admin",
		"MONGO_PASSWORD": "mongodblmpvBMCqJ3Ig2eX2oCTlNbf7TJ5533L80TvM8LC",
		"MONGO_URI":      "mongodb://localhost:27017",
		"MONGO_DB":       "tonotes",
		"MONGO_DB_TEST":  "tonotes_test",
	}

	for key, value := range requiredVars {
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	// Construct TEST_MONGO_URI
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Replace template variables in URI
	mongoURI = strings.Replace(mongoURI, "${MONGO_USERNAME}", os.Getenv("MONGO_USERNAME"), -1)
	mongoURI = strings.Replace(mongoURI, "${MONGO_PASSWORD}", os.Getenv("MONGO_PASSWORD"), -1)

	os.Setenv("TEST_MONGO_URI", mongoURI)
}

func TestTodoRepoOperations(t *testing.T) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Log environment variables
	t.Logf("Test MongoDB URI: %s", os.Getenv("TEST_MONGO_URI"))
	t.Logf("Test Database: %s", os.Getenv("MONGO_DB_TEST"))

	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Get database reference
	dbName := os.Getenv("MONGO_DB_TEST")
	if dbName == "" {
		t.Fatal("MONGO_DB_TEST environment variable not set")
	}
	db := client.Database(dbName)

	// Create todos collection
	err := db.CreateCollection(context.Background(), "todos")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Fatalf("Failed to create todos collection: %v", err)
	}

	// Verify collection exists
	collections, err := db.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}
	t.Logf("Available collections: %v", collections)

	// Initialize repository
	todoRepo := repository.TodoRepo{
		MongoCollection: db.Collection("todos"),
	}

	// Test IDs
	todoID1 := uuid.New().String()
	todoID2 := uuid.New().String()
	userID := uuid.New().String()

	t.Logf("Test IDs - Todo1: %s, Todo2: %s, User: %s", todoID1, todoID2, userID)

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Create Todo - Success",
			run: func(t *testing.T) {
				ctx := context.Background()
				todo := model.Todo{
					TodoID:      todoID1,
					UserID:      userID,
					TodoName:    "Test Todo",
					Description: "Test Description",
					Complete:    false,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
					Tags:        []string{"test"},
					Priority:    model.PriorityLow,
				}

				err := todoRepo.CreateTodo(ctx, &todo)
				if err != nil {
					t.Fatalf("Failed to create todo: %v", err)
				}

				// Verify creation
				savedTodo, err := todoRepo.GetTodosByID(userID, todoID1)
				if err != nil {
					t.Fatalf("Failed to get created todo: %v", err)
				}
				if savedTodo.TodoName != todo.TodoName {
					t.Errorf("Expected todo name %s, got %s", todo.TodoName, savedTodo.TodoName)
				}
			},
		},
		{
			name: "Create Todo - Duplicate ID",
			run: func(t *testing.T) {
				ctx := context.Background()
				todo := model.Todo{
					TodoID:      todoID1, // Using same ID as previous test
					UserID:      userID,
					TodoName:    "Duplicate Todo",
					Description: "Should fail",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				err := todoRepo.CreateTodo(ctx, &todo)
				if err == nil {
					t.Error("Expected error for duplicate todo ID, got none")
				}
			},
		},
		{
			name: "Get User Todos",
			run: func(t *testing.T) {
				ctx := context.Background()

				// Create another todo
				todo2 := model.Todo{
					TodoID:      todoID2,
					UserID:      userID,
					TodoName:    "Test Todo 2",
					Description: "Second todo",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				if err := todoRepo.CreateTodo(ctx, &todo2); err != nil {
					t.Fatalf("Failed to create second todo: %v", err)
				}

				todos, err := todoRepo.GetUserTodos(ctx, userID)
				if err != nil {
					t.Fatalf("Failed to get todos: %v", err)
				}
				if len(todos) != 2 {
					t.Errorf("Expected 2 todos, got %d", len(todos))
				}
			},
		},
		{
			name: "Update Todo",
			run: func(t *testing.T) {
				ctx := context.Background()
				updates := model.Todo{
					TodoName:    "Updated Todo",
					Description: "Updated Description",
					Complete:    true,
					UpdatedAt:   time.Now(),
					Tags:        []string{"updated"},
					Priority:    model.PriorityHigh,
				}

				err := todoRepo.UpdateTodo(ctx, todoID1, userID, &updates)
				if err != nil {
					t.Fatalf("Failed to update todo: %v", err)
				}

				// Verify update
				updated, err := todoRepo.GetTodosByID(userID, todoID1)
				if err != nil {
					t.Fatalf("Failed to get updated todo: %v", err)
				}
				if updated.TodoName != updates.TodoName {
					t.Errorf("Expected todo name %s, got %s", updates.TodoName, updated.TodoName)
				}
				if !updated.Complete {
					t.Error("Todo should be marked as complete")
				}
			},
		},
		{
			name: "Delete Todo",
			run: func(t *testing.T) {
				ctx := context.Background()
				err := todoRepo.DeleteTodo(ctx, todoID2, userID)
				if err != nil {
					t.Fatalf("Failed to delete todo: %v", err)
				}

				// Verify deletion
				_, err = todoRepo.GetTodosByID(userID, todoID2)
				if err == nil {
					t.Error("Expected error getting deleted todo, got none")
				}
			},
		},
		{
			name: "Count Todos",
			run: func(t *testing.T) {
				ctx := context.Background()
				count, err := todoRepo.CountAllTodos(ctx, userID)
				if err != nil {
					t.Fatalf("Failed to count todos: %v", err)
				}
				if count != 1 { // After creating 2 and deleting 1
					t.Errorf("Expected 1 todo, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
