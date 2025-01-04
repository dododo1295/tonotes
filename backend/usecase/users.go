package usecase

import (
	"log"

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

func NewUserService(repo repository.UsersRepo) *UserService {
	return &UserService{repo : repo
		userID : GenerateUserID(),}
}
