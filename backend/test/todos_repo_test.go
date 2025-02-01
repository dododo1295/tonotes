package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTodoRepoOperations(t *testing.T) {
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	//Test components
	todoID1 := uuid.New().String()
	todoID2 := uuid.New().String()
	userID := uuid.New().String()

	//init repo
	coll := client.Database(os.Getenv("MONGO_DB")).Collection(os.Getenv("TODOS_COLLECTION"))
	todoRepo := repository.TodosRepo{MongoCollection: coll}

	// Create todos
	t.Run("CreateFirstTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todo := model.Todos{
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
			t.Fatal("failed to create todo:", err)
		}
		t.Log("todo created successfully")
	})
	t.Run("CreateSecondTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todo := model.Todos{
			TodoID:      todoID2,
			UserID:      userID,
			TodoName:    "Test Todo 2",
			Description: "Test Description 2",
			Complete:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
			Priority:    model.PriorityLow,
		}

		err := todoRepo.CreateTodo(ctx, &todo)
		if err != nil {
			t.Fatal("failed to create todo:", err)
		}
		t.Log("todo created successfully")
	})

	// Get all todos
	t.Run("GetUserTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoRepo.GetUserTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get todos:", err)
		}
		if len(todos) == 0 {
			t.Error("expected todos, got none")
		}
		t.Log("todos retrieved successfully")
	})

	// Update todo
	t.Run("UpdateTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		updates := model.Todos{
			TodoName:    "Updated Todo",
			Description: "Updated Description",
			Complete:    true,
			UpdatedAt:   time.Now(),
			Tags:        []string{"updated"},
			Priority:    model.PriorityHigh,
		}

		err := todoRepo.UpdateTodo(ctx, todoID1, userID, &updates)
		if err != nil {
			t.Fatal("failed to update todo:", err)
		}
		t.Log("todo updated successfully")

		// Verify update
		todos, err := todoRepo.GetUserTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get updated todo:", err)
		}
		for _, todo := range todos {
			if todo.TodoID == todoID1 {
				if todo.TodoName != "Updated Todo" {
					t.Error("todo was not updated correctly")
				}
				break
			}
		}
	})

	// Delete todo
	t.Run("DeleteTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := todoRepo.DeleteTodo(ctx, todoID2, userID)
		if err != nil {
			t.Fatal("failed to delete todo:", err)
		}
		t.Log("todo deleted successfully")

		// Verify deletion
		todos, err := todoRepo.GetUserTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get todos after deletion:", err)
		}
		for _, todo := range todos {
			if todo.TodoID == todoID2 {
				t.Error("todo was not deleted")
			}
		}
	})

	// Count todos
	t.Run("CountAllTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		count, err := todoRepo.CountAllTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to count todos:", err)
		}
		t.Log("todos counted successfully:", count)
	})
}
