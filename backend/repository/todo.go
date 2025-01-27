package repository

import (
	"context"
	"errors"
	"main/model"
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
	if todo.UserID == "" {
		return errors.New("user ID is required")
	}

	_, err := r.MongoCollection.InsertOne(context.Background(), todo)
	return err
}

// GetUserTodos retrieves all todos for a user
func (r *TodosRepo) GetUserTodos(userID string) ([]*model.Todos, error) {
	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

// UpdateTodo updates a specific todo
func (r *TodosRepo) UpdateTodo(todoID string, userID string, updates *model.Todos) error {
	filter := bson.M{
		"_id":     todoID,
		"user_id": userID, // Ensure user owns this todo
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
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("todo not found")
	}

	return nil
}

// DeleteTodo removes a specific todo
func (r *TodosRepo) DeleteTodo(todoID string, userID string) error {
	filter := bson.M{
		"_id":     todoID,
		"user_id": userID, // Ensure user owns this todo
	}

	result, err := r.MongoCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("todo not found")
	}

	return nil
}

// ToggleTodoComplete toggles the complete status of a todo
func (r *TodosRepo) ToggleTodoComplete(todoID string, userID string) error {
	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	// First, get the current complete status
	var todo model.Todos
	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&todo)
	if err != nil {
		return err
	}

	// Toggle the complete status
	update := bson.M{
		"$set": bson.M{
			"complete":   !todo.Complete,
			"updated_at": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("todo not found")
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
