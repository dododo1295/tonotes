package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/usecase"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func init() {
	testutils.SetupTestEnvironment()
}

func setupTodosUsecaseTest(t *testing.T) (*repository.TodosRepo, *usecase.TodosService, func()) {
	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)

	// Get database reference
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Create todos collection
	err := db.CreateCollection(context.Background(), "todos")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Logf("Warning: Failed to create todos collection: %v", err)
	}

	// Initialize repository and service
	todosRepo := &repository.TodosRepo{
		MongoCollection: db.Collection("todos"),
	}
	todosService := usecase.NewTodosService(todosRepo)

	return todosRepo, todosService, func() {
		if err := db.Collection("todos").Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop todos collection: %v", err)
		}
		cleanup()
	}
}

func TestTodosService(t *testing.T) {
	repo, service, cleanup := setupTodosUsecaseTest(t)
	defer cleanup()

	userID := uuid.New().String()

	tests := []struct {
		name    string
		run     func(t *testing.T)
		cleanup func(t *testing.T)
	}{
		{
			name: "Search Todos",
			run: func(t *testing.T) {
				ctx := context.Background()

				// Create test todos
				todos := []*model.Todos{
					{
						TodoID:      uuid.New().String(),
						UserID:      userID,
						TodoName:    "Test High Priority",
						Description: "Important task",
						Complete:    false,
						Priority:    model.PriorityHigh,
						Tags:        []string{"important", "urgent"},
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						TodoID:      uuid.New().String(),
						UserID:      userID,
						TodoName:    "Test Low Priority",
						Description: "Regular task",
						Complete:    false,
						Priority:    model.PriorityLow,
						Tags:        []string{"regular"},
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				}

				for _, todo := range todos {
					if err := repo.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create test todo: %v", err)
					}
				}

				// Test search by query
				results, err := service.SearchTodos(ctx, userID, "Important")
				if err != nil {
					t.Errorf("Failed to search todos: %v", err)
				}
				if len(results) != 1 {
					t.Errorf("Expected 1 result, got %d", len(results))
				}
			},
			cleanup: func(t *testing.T) {
				ctx := context.Background()
				if err := repo.MongoCollection.Drop(ctx); err != nil {
					t.Logf("Warning: Failed to drop collection: %v", err)
				}
			},
		},
		{
			name: "Filter By Priority",
			run: func(t *testing.T) {
				ctx := context.Background()

				// Create todos with different priorities
				todos := []*model.Todos{
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "High Priority Task",
						Priority:  model.PriorityHigh,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Medium Priority Task",
						Priority:  model.PriorityMedium,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Low Priority Task",
						Priority:  model.PriorityLow,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}

				for _, todo := range todos {
					if err := repo.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create test todo: %v", err)
					}
				}

				// Test filtering by priority
				highPriorityTodos, err := service.GetTodosByPriority(ctx, userID, model.PriorityHigh)
				if err != nil {
					t.Errorf("Failed to get high priority todos: %v", err)
				}
				if len(highPriorityTodos) != 1 {
					t.Errorf("Expected 1 high priority todo, got %d", len(highPriorityTodos))
				}
			},
			cleanup: func(t *testing.T) {
				ctx := context.Background()
				if err := repo.MongoCollection.Drop(ctx); err != nil {
					t.Logf("Warning: Failed to drop collection: %v", err)
				}
			},
		},
		{
			name: "Filter By Tags",
			run: func(t *testing.T) {
				ctx := context.Background()

				// Create todos with different tags
				todos := []*model.Todos{
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Work Task",
						Tags:      []string{"work", "important"},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Personal Task",
						Tags:      []string{"personal"},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}

				for _, todo := range todos {
					if err := repo.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create test todo: %v", err)
					}
				}

				// Test filtering by tags
				workTodos, err := service.GetTodosByTags(ctx, userID, []string{"work"})
				if err != nil {
					t.Errorf("Failed to get todos by tags: %v", err)
				}
				if len(workTodos) != 1 {
					t.Errorf("Expected 1 work todo, got %d", len(workTodos))
				}
			},
			cleanup: func(t *testing.T) {
				ctx := context.Background()
				if err := repo.MongoCollection.Drop(ctx); err != nil {
					t.Logf("Warning: Failed to drop collection: %v", err)
				}
			},
		},
		{
			name: "Get Due Todos",
			run: func(t *testing.T) {
				ctx := context.Background()

				// Create todos with different due dates
				todos := []*model.Todos{
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Overdue Task",
						DueDate:   time.Now().Add(-24 * time.Hour),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						TodoID:    uuid.New().String(),
						UserID:    userID,
						TodoName:  "Future Task",
						DueDate:   time.Now().Add(24 * time.Hour),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}

				for _, todo := range todos {
					if err := repo.CreateTodo(ctx, todo); err != nil {
						t.Fatalf("Failed to create test todo: %v", err)
					}
				}

				// Test getting overdue todos
				overdueTodos, err := service.GetOverdueTodos(ctx, userID)
				if err != nil {
					t.Errorf("Failed to get overdue todos: %v", err)
				}
				if len(overdueTodos) != 1 {
					t.Errorf("Expected 1 overdue todo, got %d", len(overdueTodos))
				}
			},
			cleanup: func(t *testing.T) {
				ctx := context.Background()
				if err := repo.MongoCollection.Drop(ctx); err != nil {
					t.Logf("Warning: Failed to drop collection: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.cleanup(t)
			tt.run(t)
		})
	}
}
