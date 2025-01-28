package repository

import (
	"context"
	"errors"
	"main/model"
	"main/utils"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type TodosRepo struct {
	MongoCollection *mongo.Collection
}

// Constructor function for TodosRepo
func GetTodosRepo(client *mongo.Client) *TodosRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("TODOS_COLLECTION")
	return &TodosRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

// CreateTodo adds a new todo
func (r *TodosRepo) CreateTodo(todo *model.Todos) error {
	timer := utils.TrackDBOperation("insert", "todos")
	defer timer.ObserveDuration()

	if todo.UserID == "" {
		utils.TrackError("database", "missing_user_id")
		return errors.New("user ID is required")
	}

	_, err := r.MongoCollection.InsertOne(context.Background(), todo)
	if err != nil {
		utils.TrackError("database", "todo_creation_failed")
		return err
	}

	return nil
}

// GetUserTodos retrieves all todos for a user
func (r *TodosRepo) GetUserTodos(userID string) ([]*model.Todos, error) {
	timer := utils.TrackDBOperation("find", "todos")
	defer timer.ObserveDuration()

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID})
	if err != nil {
		utils.TrackError("database", "todo_fetch_failed")
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &todos); err != nil {
		utils.TrackError("database", "todo_decode_failed")
		return nil, err
	}
	return todos, nil
}

// UpdateTodo updates a specific todo
func (r *TodosRepo) UpdateTodo(todoID string, userID string, updates *model.Todos) error {
	timer := utils.TrackDBOperation("update", "todos")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"todo_name":        updates.TodoName,
			"todo_description": updates.TodoDescription,
			"complete":         updates.Complete,
			"updated_at":       time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "todo_update_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "todo_not_found")
		return errors.New("todo not found")
	}

	// Track todo completion if the update includes setting complete to true
	if updates.Complete {
		utils.TrackTodoCompletion(userID)
	}

	return nil
}

// DeleteTodo removes a specific todo
func (r *TodosRepo) DeleteTodo(todoID string, userID string) error {
	timer := utils.TrackDBOperation("delete", "todos")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	result, err := r.MongoCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		utils.TrackError("database", "todo_deletion_failed")
		return err
	}

	if result.DeletedCount == 0 {
		utils.TrackError("database", "todo_not_found")
		return errors.New("todo not found")
	}

	return nil
}

// ToggleTodoComplete toggles the complete status of a todo
func (r *TodosRepo) ToggleTodoComplete(todoID string, userID string) error {
	timer := utils.TrackDBOperation("update", "todos")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	var todo model.Todos
	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&todo)
	if err != nil {
		utils.TrackError("database", "todo_not_found")
		return err
	}

	// Toggle the complete status
	newCompleteStatus := !todo.Complete
	update := bson.M{
		"$set": bson.M{
			"complete":   newCompleteStatus,
			"updated_at": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "todo_update_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "todo_not_found")
		return errors.New("todo not found")
	}

	// Track completion if the todo was marked as complete
	if newCompleteStatus {
		utils.TrackTodoCompletion(userID)
	}

	return nil
}

// CountUserTodos counts the number of todos for a user
func (r *TodosRepo) CountUserTodos(userID string) (int, error) {
	count, err := r.MongoCollection.CountDocuments(context.Background(),
		bson.M{"user_id": userID})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetCompletedTodos gets all completed todos for a user
func (r *TodosRepo) GetCompletedTodos(userID string) ([]*model.Todos, error) {
	filter := bson.M{
		"user_id":  userID,
		"complete": true,
	}

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

// GetPendingTodos gets all pending todos for a user
func (r *TodosRepo) GetPendingTodos(userID string) ([]*model.Todos, error) {
	filter := bson.M{
		"user_id":  userID,
		"complete": false,
	}

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &todos); err != nil {
		return nil, err
	}
	return todos, nil
}
