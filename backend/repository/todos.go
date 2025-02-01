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

// Retrieves MongoDB collection for todos
func GetTodosRepo(client *mongo.Client) *TodosRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("TODOS_COLLECTION")
	return &TodosRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

// Add a new todo (following the model) into the database
func (r *TodosRepo) CreateTodo(ctx context.Context, todo *model.Todos) error {
	timer := utils.TrackDBOperation("insert", "todos")
	defer timer.ObserveDuration()

	if todo.UserID == "" {
		utils.TrackError("database", "missing_user_id")
		return errors.New("user ID is required")
	}

	validTags, err := validateTags(todo.Tags)
	if err != nil {
		return err
	}
	todo.Tags = validTags

	_, err = r.MongoCollection.InsertOne(ctx, todo)
	if err != nil {
		utils.TrackError("database", "todo_creation_failed")
		return err
	}

	return nil
}

// Retrieves all todos based on the User ID
func (r *TodosRepo) GetUserTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	timer := utils.TrackDBOperation("find", "todos")
	defer timer.ObserveDuration()

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(ctx,
		bson.M{"user_id": userID})
	if err != nil {
		utils.TrackError("database", "todo_fetch_failed")
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &todos); err != nil {
		utils.TrackError("database", "todo_decode_failed")
		return nil, err
	}
	return todos, nil
}

// All encompassing update for a specific todo (Name, Description, Complete status)
func (r *TodosRepo) UpdateTodo(ctx context.Context, todoID string, userID string, updates *model.Todos) error {
	timer := utils.TrackDBOperation("update", "todos")
	defer timer.ObserveDuration()

	validTags, err := validateTags(updates.Tags)
	if err != nil {
		return err
	}
	updates.Tags = validTags

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"todo_name":        updates.TodoName,
			"todo_description": updates.Description,
			"complete":         updates.Complete,
			"updated_at":       time.Now(),
			"tags":             updates.Tags,
		},
	}

	result, err := r.MongoCollection.UpdateOne(ctx, filter, update)
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

// Removes a specific todo from database
func (r *TodosRepo) DeleteTodo(ctx context.Context, todoID string, userID string) error {
	timer := utils.TrackDBOperation("delete", "todos")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	result, err := r.MongoCollection.DeleteOne(ctx, filter)
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

// Toggles the complete status of a todo
func (r *TodosRepo) ToggleTodoComplete(ctx context.Context, todoID string, userID string) error {
	timer := utils.TrackDBOperation("update", "todos")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     todoID,
		"user_id": userID,
	}

	var todo model.Todos
	err := r.MongoCollection.FindOne(ctx, filter).Decode(&todo)
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

	result, err := r.MongoCollection.UpdateOne(ctx, filter, update)
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

// Counts the non-sorted number of todos for a user for display in the UI
func (r *TodosRepo) CountAllTodos(ctx context.Context, userID string) (int, error) {
	count, err := r.MongoCollection.CountDocuments(ctx,
		bson.M{"user_id": userID})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Counts the number of pending todos for a user for display in the UI
func (r *TodosRepo) PendingCount(ctx context.Context, userID string) (int, error) {
	timer := utils.TrackDBOperation("count", "pending_todos")
	defer timer.ObserveDuration()

	count, err := r.MongoCollection.CountDocuments(ctx,
		bson.M{"user_id": userID, "complete": false})
	if err != nil {
		utils.TrackError("database", "pending_todo_count_failed")
		return 0, err
	}
	return int(count), nil
}

// Counts the number of completed todos for a user for display in the UI
func (r *TodosRepo) CompletedCount(ctx context.Context, userID string) (int, error) {
	timer := utils.TrackDBOperation("count", "completed_todos")
	defer timer.ObserveDuration()

	count, err := r.MongoCollection.CountDocuments(ctx,
		bson.M{"user_id": userID, "complete": true})
	if err != nil {
		utils.TrackError("database", "completed_todo_count_failed")
	}
	return int(count), nil
}

// Gets all completed todos for a user
func (r *TodosRepo) GetCompletedTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	filter := bson.M{
		"user_id":  userID,
		"complete": true,
	}

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

// Gets all pending todos for a user
func (r *TodosRepo) GetPendingTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	filter := bson.M{
		"user_id":  userID,
		"complete": false,
	}

	var todos []*model.Todos
	cursor, err := r.MongoCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

// helper functions

func validateTags(tags []string) ([]string, error) {
	if tags == nil || len(tags) == 0 {
		return nil, nil
	}
	var validTags []string
	for _, tag := range tags {
		if tag != "" {
			validTags = append(validTags, tag)
		}
	}
	if len(validTags) > 5 {
		return nil, errors.New("cannot exceed 5 tags per todo")
	}

	for _, tag := range validTags {
		if len(tag) > 20 {
			return nil, errors.New("tag cannot exceed 20 characters")
		}
	}

	return validTags, nil
}
