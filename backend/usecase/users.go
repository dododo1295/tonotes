package usecase

import (
	"main/repository"
)

type UserService struct {
	repo repository.UsersRepo
}
