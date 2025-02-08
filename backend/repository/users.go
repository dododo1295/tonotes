package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"main/model"
	"main/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetUserRepo(client *mongo.Client) *UserRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("USERS_COLLECTION")
	return &UserRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

type UserRepo struct {
	MongoCollection *mongo.Collection
}

func (r *UserRepo) AddUser(ctx context.Context, user *model.User) (interface{}, error) {
	timer := utils.TrackDBOperation("insert", "users")
	defer timer.ObserveDuration()

	if user.Username == "" || user.Password == "" {
		utils.TrackError("database", "invalid_user_data")
		return nil, errors.New("username and password required")
	}

	result, err := r.MongoCollection.InsertOne(ctx, user)
	if err != nil {
		utils.TrackError("database", "user_creation_failed")
		return nil, errors.New("failed to add user to database")
	}

	utils.TrackRegistration()
	return result.InsertedID, nil
}

func (r *UserRepo) FindUserByUsername(username string) (*model.User, error) {
	timer := utils.TrackDBOperation("find", "users")
	defer timer.ObserveDuration()

	var user model.User
	filter := bson.D{{Key: "username", Value: username}}

	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.TrackError("database", "user_not_found")
			return nil, nil
		}
		utils.TrackError("database", "user_lookup_error")
		log.Println("Error finding user:", err)
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) FindUser(userID string) (*model.User, error) {
    timer := utils.TrackDBOperation("find", "users")
    defer timer.ObserveDuration()

    var user model.User
    filter := bson.D{{Key: "user_id", Value: userID}}

    // Add logging for debugging
    log.Printf("Finding user with ID: %s", userID)

    // Explicitly set all fields in projection
    opts := options.FindOne().SetProjection(bson.M{
        "user_id": 1,
        "username": 1,
        "email": 1,
        "password": 1,
        "createdAt": 1,
        "lastEmailChange": 1,
        "lastPasswordChange": 1,
        "is_active": 1,
        "two_factor_secret": 1,
        "two_factor_enabled": 1,
        "recovery_codes": 1,
    })

    err := r.MongoCollection.FindOne(context.Background(), filter, opts).Decode(&user)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            utils.TrackError("database", "user_not_found")
            return nil, nil
        }
        utils.TrackError("database", "user_lookup_error")
        return nil, err
    }

    // Add logging for debugging
    log.Printf("Found user: %+v", user)
    log.Printf("Recovery codes: %v", user.RecoveryCodes)

    return &user, nil
}

func (r *UserRepo) UpdateUserPassword(userID string, hashedPassword string) (int64, error) {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

	if hashedPassword == "" {
		utils.TrackError("database", "invalid_password_hash")
		return 0, fmt.Errorf("password hashing error")
	}

	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"password":           hashedPassword,
			"lastPasswordChange": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "password_update_failed")
		return 0, fmt.Errorf("failed to update password: %w", err)
	}

	return result.ModifiedCount, nil
}

func (r *UserRepo) UpdateUserByID(userID string, updateID *model.User) (int64, error) {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

	filter := bson.D{{Key: "user_id", Value: userID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "username", Value: updateID.Username},
		}},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "username_update_failed")
		return 0, err
	}

	return result.ModifiedCount, nil
}

func (r *UserRepo) DeleteUserByID(userID string) (int64, error) {
	timer := utils.TrackDBOperation("delete", "users")
	defer timer.ObserveDuration()

	result, err := r.MongoCollection.DeleteOne(context.Background(),
		bson.D{{Key: "user_id", Value: userID}})
	if err != nil {
		utils.TrackError("database", "user_deletion_failed")
		return 0, err
	}

	return result.DeletedCount, nil
}

func (r *UserRepo) UpdateUserEmail(userID string, email string) (int64, error) {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"email":           email,
			"lastEmailChange": time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "email_update_failed")
		return 0, fmt.Errorf("failed to update email: %w", err)
	}

	return result.ModifiedCount, nil
}

func (r *UserRepo) Enable2FAWithRecoveryCodes(userID, secret string, recoveryCodes []string) error {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

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
		utils.TrackError("database", "2fa_enable_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "user_not_found")
		return errors.New("user not found")
	}

	return nil
}

func (r *UserRepo) UpdateRecoveryCodes(userID string, codes []string) error {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"recovery_codes": codes,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "recovery_codes_update_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "user_not_found")
		return errors.New("user not found")
	}

	return nil
}

func (r *UserRepo) Disable2FA(userID string) error {
	timer := utils.TrackDBOperation("update", "users")
	defer timer.ObserveDuration()

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
		utils.TrackError("database", "2fa_disable_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "user_not_found")
		return errors.New("user not found")
	}

	return nil
}
