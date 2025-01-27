package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"main/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// get usersrepo from database
var mongoClient *mongo.Client

// Constructor function for UsersRepo
func GetUsersRepo(client *mongo.Client) *UsersRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("USERS_COLLECTION")
	return &UsersRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

// Getting DB
type UsersRepo struct {
	MongoCollection *mongo.Collection
}

// Creating User
func (r *UsersRepo) AddUser(ctx context.Context, user *model.User) (interface{}, error) {
	if user.Username == "" || user.Password == "" {
		return nil, errors.New("username and password required")
	}

	result, err := r.MongoCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, errors.New("failed to add user to database")
	}
	return result.InsertedID, nil
}

// Finding via username
func (r *UsersRepo) FindUserByUsername(username string) (*model.User, error) {
	var user model.User
	filter := bson.D{{Key: "username", Value: username}}

	// Find the user by username using the repo's collection
	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // User not found
		}
		log.Println("Error finding user:", err)
		return nil, err
	}

	return &user, nil
}

// Finding UserID
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
	filter := bson.M{"user_id": userID}
	// update time!
	update := bson.M{
		"$set": bson.M{
			"password":           hashedPassword,
			"lastPasswordChange": time.Now(),
		},
	}
	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to update password: %w", err)
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

// Updating email

func (r *UsersRepo) UpdateUserEmail(userID string, email string) (int64, error) {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"email":           email,
			"lastEmailChange": time.Now(),
		},
	}
	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to update email: %w", err)
	}
	return result.ModifiedCount, nil
}

func (r *UsersRepo) Enable2FAWithRecoveryCodes(userID, secret string, recoveryCodes []string) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"two_factor_secret":  secret,
			"two_factor_enabled": true,
			"recovery_codes":     recoveryCodes,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (r *UsersRepo) UpdateRecoveryCodes(userID string, codes []string) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"recovery_codes": codes,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (r *UsersRepo) Disable2FA(userID string) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"two_factor_secret":  "",
			"two_factor_enabled": false,
			"recovery_codes":     nil,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}
