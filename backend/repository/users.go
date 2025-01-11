package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"main/model"
	"main/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// NOTE: This repo STILL does not have the ability to find username via email
// Getting DB
type UsersRepo struct {
	MongoCollection *mongo.Collection
}

// Creating User
func (r *UsersRepo) AddUser(ctx context.Context, user *model.User) (interface{}, error) {
	if user.Username == "" || user.Password == "" {
		return nil, errors.New("username and password required")
	}
	hashedPassword, err := services.HashPassword(user.Password)
	if err != nil {
		return nil, errors.New("failed to hash")
	}

	user.Password = hashedPassword

	user.CreatedAt = time.Now()

	result, err := r.MongoCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, errors.New("failed to add user to database")
	}
	return result.InsertedID, nil
}

// Finding User
func (r *UsersRepo) FindUser(userID string) (*model.User, error) {
	var user model.User
	err := r.MongoCollection.FindOne(context.Background(),
		bson.D{{Key: "user_id", Value: userID}}).Decode(&user)
	if err != nil {
		// Handle the case where no document is found
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// Updating User Password
// hashing on the front end, DUH
func (r *UsersRepo) UpdateUserPassword(userID string, hashedPassword string) (int64, error) {
	// return error if the password isn't hashed properly
	if hashedPassword == "" {
		return 0, fmt.Errorf("password hashing error")
	}
	// filter the ID
	filter := bson.D{{Key: "user_id", Value: userID}}
	// update time!
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "password", Value: hashedPassword}}}}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// Updating Username
func (r *UsersRepo) UpdateUserByID(userID string, updateID *model.User) (int64, error) {
	filter := bson.D{{Key: "user_id", Value: userID}}
	// setting to only update the username
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "username", Value: updateID.Username},
		}},
	}
	// update now

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// Deleting Users
func (r *UsersRepo) DeleteUserByID(userID string) (int64, error) {
	result, err := r.MongoCollection.DeleteOne(context.Background(),
		bson.D{{Key: "user_id", Value: userID}})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
