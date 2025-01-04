package usecase

import (
	"log"
	"main/model"
	"main/repository"

	"github.com/google/uuid"
)

type UserService struct {
	repo repository.UsersRepo
}

func GenerateUserID() string {
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal("Failed to generate a unique ID", err)
	}
	return id.String()
}

// CreateUser is a function to create a new user
func CreateUser(input model.User (models.Users, error) {
	// Generate a new UserID
	userID, err := GenerateUserID()
	if err != nil {
		return model.Users{}, err // Handle error if UserID generation fails
	}

	// Populate the Users struct with generated UserID
	newUser := model.Users{
		UserID:   userID, // Assign the generated ID here
		Name:     input.Name,
		Email:    input.Email,
		Password: input.Password, // Assuming it's hashed
	}

	// Perform any necessary database operations here
	err = saveUserToDB(newUser) // Replace with your actual DB call
	if err != nil {
		return models.Users{}, err
	}

	return newUser, nil
}
