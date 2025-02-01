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
	todoID3 := uuid.New().String()
	todoID4 := uuid.New().String()
	userID := uuid.New().String()

	//init repo
	coll := client.Database(os.Getenv("MONGO_DB")).Collection(os.Getenv("TODOS_COLLECTION"))
	todoRepo := repository.TodosRepo{MongoCollection: coll}

	// create todos
	t.Run("CreateFirstTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todo := model.Todos{
			TodoID:      todoID1,
			UserID:      userID,
			TodoName:    "this is a test",
			Description: "this is a test",
			Complete:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
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
			TodoName:    "this is a test 2",
			Description: "this is a test 2",
			Complete:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
		}

		err := todoRepo.CreateTodo(ctx, &todo)
		if err != nil {
			t.Fatal("failed to create todo:", err)
		}
		t.Log("todo created successfully")
	})

	t.Run("CreateThirdTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todo := model.Todos{
			TodoID:      todoID3,
			UserID:      userID,
			TodoName:    "this is a test 3",
			Description: "this is a test 3",
			Complete:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
		}

		err := todoRepo.CreateTodo(ctx, &todo)
		if err != nil {
			t.Fatal("failed to create todo:", err)
		}
		t.Log("todo created successfully")
	})

	t.Run("CreateFourthTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todo := model.Todos{
			TodoID:      todoID4,
			UserID:      userID,
			TodoName:    "this is a test 4",
			Description: "this is a test 4",
			Complete:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
		}

		err := todoRepo.CreateTodo(ctx, &todo)
		if err != nil {
			t.Fatal("failed to create todo:", err)
		}
		t.Log("todo created successfully")
	})

	// get all todos
	t.Run("GetUserTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoRepo.GetUserTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get todos:", err)
		}
		t.Log("todos retrieved successfully:", todos)
	})

	//updating a todo
	t.Run("UpdateTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		updates := model.Todos{
			TodoName:    "this is a test 4",
			Description: "this is a test 444444444",
			Complete:    true,
			UpdatedAt:   time.Now(),
			Tags:        []string{"test"},
		}

		err := todoRepo.UpdateTodo(ctx, todoID1, userID, &updates)
		if err != nil {
			t.Fatal("failed to update todo:", err)
		}
		t.Log("todo updated successfully:", updates)
	})

	// delete a todo
	t.Run("DeleteTodo", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := todoRepo.DeleteTodo(ctx, todoID2, userID)
		if err != nil {
			t.Fatal("failed to delete todo:", err)
		}
		t.Log("todo #2 deleted successfully")
	})

	// toggle complete status
	t.Run("ToggleCompleteStatus", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := todoRepo.ToggleTodoComplete(ctx, todoID3, userID)
		if err != nil {
			t.Fatal("failed to toggle complete status:", err)
		}
		t.Log("todo #1 complete status toggled successfully:")
	})

	// count all todos
	t.Run("CountTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		count, err := todoRepo.CountAllTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to count todos:", err)
		}
		t.Log("todos count:", count)
	})

	// count completed todos
	t.Run("CompleteTodosCount", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		count, err := todoRepo.CompletedCount(ctx, userID)
		if err != nil {
			t.Fatal("failed to count completed todos:", err)
		}
		t.Log("completed todos count:", count)
	})

	// count pending todos
	t.Run("PendingTodosCount", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		count, err := todoRepo.PendingCount(ctx, userID)
		if err != nil {
			t.Fatal("failed to count pending todos:", err)
		}
		t.Log("pending todos count:", count)
	})
	// get all completed
	t.Run("GetAllCompletedTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoRepo.GetCompletedTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get completed todos:", err)
		}
		t.Log("completed todos retrieved successfully:", todos)
	})

	// get all pending
	t.Run("GetAllPendingTodos", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		todos, err := todoRepo.GetPendingTodos(ctx, userID)
		if err != nil {
			t.Fatal("failed to get pending todos:", err)
		}
		t.Log("pending todos retrieved successfully:", todos)
	})
}
