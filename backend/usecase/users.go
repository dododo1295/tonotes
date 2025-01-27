package usecase

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"main/model"
	"main/repository"
	"main/services"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserService struct {
	UsersRepo *repository.UserRepo
}

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// creating user and inserting uuid
func (svc *UserService) CreateUser(c *gin.Context, user *model.User) error {
	// Generate UUID for new user
	user.UserID = uuid.NewString()

	// Hash the password
	hashedPassword, err := services.HashPassword(user.Password)
	if err != nil {
		return err
	}
	//set password
	user.Password = hashedPassword

	// Set creation time
	user.CreatedAt = time.Now()

	// Check if username already exists
	existingUser, err := svc.UsersRepo.FindUserByUsername(user.Username)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return fmt.Errorf("username already exists")
	}

	// Add user to database
	_, err = svc.UsersRepo.AddUser(c, user)
	if err != nil {
		return err
	}

	return nil
}

func (svc *UserService) GetUserProfile(userID string) (*model.UserProfile, error) {
	if userID == "" {
		return nil, fmt.Errorf("invalid user id")
	}

	user, err := svc.UsersRepo.FindUser(userID)
	if err != nil {
		return nil, fmt.Errorf("could not find user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return &model.UserProfile{
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (svc *UserService) UpdateUserPassword(userID string, oldPassword, newPassword string) error {
	// Business logic only
	user, err := svc.UsersRepo.FindUser(userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	if !services.ComparePasswords(user.Password, oldPassword) {
		return fmt.Errorf("current password incorrect")
	}

	if !utils.ValidatePassword(newPassword) {
		return fmt.Errorf("password does not meet requirements")
	}

	if services.ComparePasswords(user.Password, newPassword) {
		return fmt.Errorf("new password same as current")
	}

	if time.Since(user.LastPasswordChange) < 14*24*time.Hour {
		return fmt.Errorf("password can only be changed every 2 weeks")
	}

	hashedPassword, err := services.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to process new password: %w", err)
	}

	_, err = svc.UsersRepo.UpdateUserPassword(userID, hashedPassword)
	return err
}

func (svc *UserService) UpdateUserByID(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	res := &Response{}

	userID := c.Param("user_id")
	log.Println("user ID: ", userID)

	if userID == "" {
		c.JSON(http.StatusBadRequest, Response{Error: "invalid user id"})
		log.Println("invalid user ID")
		return
	}

	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "could not bind: " + err.Error()})
		log.Println("error decoding")
		return
	}

	user.UserID = userID

	count, err := svc.UsersRepo.UpdateUserByID(userID, &user)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "could not update user name"})
		log.Println("error updating usernmae")
		return
	}

	res.Data = count
	c.JSON(http.StatusOK, res)
}

func (svc *UserService) FindUserByUsername(username string) (*model.User, error) {
	// Here you could add business logic before/after the database call
	return svc.UsersRepo.FindUserByUsername(username)
}
