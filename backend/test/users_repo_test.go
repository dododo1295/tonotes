package test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"main/model"
	"main/repository"
	"main/test/testutils"

	"github.com/google/uuid"
)

func TestUserRepoOperations(t *testing.T) {
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	user1 := uuid.New().String()
	user2 := uuid.New().String()

	// Use environment variables for database and collection names
	coll := client.Database(os.Getenv("MONGO_DB")).Collection(os.Getenv("USERS_COLLECTION"))
	userRepo := repository.UserRepo{MongoCollection: coll}

	// Adding Users!
	t.Run("CreateUser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user := model.User{
			UserID:    user1,
			Username:  "testUser",
			Email:     "testemail@email.com",
			Password:  "12341234",
			CreatedAt: time.Now(),
		}

		result, err := userRepo.AddUser(ctx, &user)
		if err != nil {
			t.Fatal("add user failed!", err)
		}
		t.Log("add user success!", result)
	})

	t.Run("CreateUser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user := model.User{
			UserID:    user2,
			Username:  "testUser2",
			Email:     "testemail2@email.com",
			Password:  "123412342",
			CreatedAt: time.Now(),
		}

		result, err := userRepo.AddUser(ctx, &user)
		if err != nil {
			t.Fatal("add user failed!", err)
		}
		t.Log("add user success!", result)
	})

	// changing the password of user1

	t.Run("UpdateUserPassword", func(t *testing.T) {
		newPassword := "success!!"
		_, err := userRepo.UpdateUserPassword(user1, newPassword)
		if err != nil {
			t.Fatal("failed to update password")
		}
		t.Log("Password changed!")
	})

	// finding the User!
	t.Run("FindingUsers", func(t *testing.T) {
		user, err := userRepo.FindUser(user1)
		if err != nil {
			t.Fatal("couldn't get the user", err)
		}
		t.Log("user id: ", user)
	})

	// time to update user1

	t.Run("UpdateUserByID", func(t *testing.T) {
		user := model.User{
			UserID:   user1,
			Username: "just_once_more",
		}
		result, err := userRepo.UpdateUserByID(user2, &user)
		if err != nil {
			log.Fatal("failed to update username")
		}
		t.Log("updated name: ", result)
	})
	// deleting user 2
	t.Run("DeleteUserByID", func(t *testing.T) {
		result, err := userRepo.DeleteUserByID(user1)
		if err != nil {
			t.Fatal("deleting failed", err)
		}

		t.Log("deleted!", result)
	})

	// updating email
	t.Run("UpdateUserEmailByUsername", func(t *testing.T) {
		result, err := userRepo.UpdateUserEmail(user1, "testemail2@email.com")
		if err != nil {
			t.Fatal("failed to update email", err)
		}
		t.Log("email updated!", result)
	})
}
