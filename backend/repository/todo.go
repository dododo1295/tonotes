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

type TodoRepo struct {
	MongoCollection *mongo.Collection
}

func GetTodoRepo(client *mongo.Client) *TodoRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("TODO_COLLECTION")
	return &TodoRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

func (r *TodoRepo) CreateTodo(ctx context.Context, todo *model.Todo) error {
	timer := utils.TrackDBOperation("insert", "todos")
	defer timer.ObserveDuration()

	if todo.UserID == "" {
		utils.TrackError("database", "missing_user_id")
		return errors.New("user ID is required")
	}

	_, err := r.MongoCollection.InsertOne(ctx, todo)
	if err != nil {
		utils.TrackError("database", "todo_creation_failed")
		return err
	}

	return nil
}

func (r *TodoRepo) GetUserTodos(ctx context.Context, userID string) ([]*model.Todo, error) {
	timer := utils.TrackDBOperation("find", "todos")
	defer timer.ObserveDuration()

	var todos []*model.Todo
	cursor, err := r.MongoCollection.Find(ctx, bson.M{"user_id": userID})
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

func (r *TodoRepo) GetTodosByID(userID string, todoID string) (*model.Todo, error) {
	var todo model.Todo
	err := r.MongoCollection.FindOne(context.Background(),
		bson.M{"_id": todoID, "user_id": userID}).Decode(&todo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("note not found")
		}
		return nil, err
	}
	return &todo, nil
}

func (r *TodoRepo) UpdateTodo(ctx context.Context, todoID string, userID string, updates *model.Todo) error {
	timer := utils.TrackDBOperation("update", "todos")
	defer timer.ObserveDuration()

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
			"priority":         updates.Priority,
			"due_date":         updates.DueDate,
			"reminder_at":      updates.ReminderAt,
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

	return nil
}

func (r *TodoRepo) DeleteTodo(ctx context.Context, todoID string, userID string) error {
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

func (r *TodoRepo) CountAllTodos(ctx context.Context, userID string) (int, error) {
	timer := utils.TrackDBOperation("count", "todos")
	defer timer.ObserveDuration()

	count, err := r.MongoCollection.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		utils.TrackError("database", "todo_count_failed")
		return 0, err
	}
	return int(count), nil
}
