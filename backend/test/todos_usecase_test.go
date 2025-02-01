package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/usecase"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTodoService(t *testing.T) {
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Initialize repository and service
	repo := repository.GetTodosRepo(client)
	todoService := usecase.NewTodosService(repo)
	userID := uuid.New().String()

	// Setup test data
	setupTestTodos(t, repo, userID)

	// Test search functionality
	t.Run("SearchTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := todoService.SearchTodos(ctx, userID, "test")
		if err != nil {
			t.Fatal("failed to search todos:", err)
		}
		t.Log("search completed successfully:", len(results))
	})

	// Test priority filtering
	t.Run("GetTodosByPriority", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoService.GetTodosByPriority(ctx, userID, model.PriorityHigh)
		if err != nil {
			t.Fatal("failed to get todos by priority:", err)
		}
		t.Log("priority filtering completed successfully:", len(todos))
	})

	// Test tag filtering
	t.Run("GetTodosByTags", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoService.GetTodosByTags(ctx, userID, []string{"test"})
		if err != nil {
			t.Fatal("failed to get todos by tags:", err)
		}
		t.Log("tag filtering completed successfully:", len(todos))
	})

}

// Helper
func setupTestTodos(t *testing.T, repo *repository.TodosRepo, userID string) {
	ctx := context.Background()
	todos := []model.Todos{
		{
			TodoID:      uuid.New().String(),
			UserID:      userID,
			TodoName:    "Test High Priority",
			Description: "Test Description",
			Priority:    model.PriorityHigh,
			Tags:        []string{"test", "high"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			TodoID:      uuid.New().String(),
			UserID:      userID,
			TodoName:    "Test Low Priority",
			Description: "Test Description",
			Priority:    model.PriorityLow,
			Tags:        []string{"test", "low"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		// Add more test todos as needed
	}

	for _, todo := range todos {
		err := repo.CreateTodo(ctx, &todo)
		if err != nil {
			t.Fatal("failed to create test todo:", err)
		}
	}
}
