package repository

import (
	"context"
	"fmt"

	"main/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Getting DB
type UsersRepo struct {
	MongoCollection *mongo.Collection
}

// Creating User
func (r *UsersRepo) InsertUser(user *model.User) (interface{}, error) {
	result, err := r.MongoCollection.InsertOne(context.Background(), user)
	if err != nil {
		return nil, err
	}
	return result.InsertedID, nil
}

// Finding User
func (r *UsersRepo) FindUser(userID string) (*model.User, error) {
	var usr model.User
	err := r.MongoCollection.FindOne(context.Background(),
		bson.D{{Key: "user_id", Value: userID}}).Decode(&usr)
	if err != nil {
		return nil, err
	}
	return &usr, nil
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
	result, err := r.MongoCollection.UpdateOne(context.Background(),
		bson.D{{Key: "user_id", Value: userID}},
		bson.D{{Key: "$set", Value: updateID}})
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
