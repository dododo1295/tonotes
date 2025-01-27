package handler

import (
	"main/model"
	"main/repository"
	"main/utils"
	"os"

	"main/services"
	"main/usecase"

	"fmt"

	"github.com/gin-gonic/gin"
)

func RegistrationHandler(c *gin.Context) {
	var user model.User

	// Debug logging
	fmt.Printf("GO_ENV: %s\n", os.Getenv("GO_ENV"))
	fmt.Printf("Database: %s\n", os.Getenv("MONGO_DB"))

	// Bind JSON and validate request
	if err := c.ShouldBindJSON(&user); err != nil {
		fmt.Printf("Bind error: %v\n", err)
		utils.BadRequest(c, "invalid request")
		return
	}

	// Debug logging
	fmt.Printf("User data: %+v\n", user)

	// Select database based on environment
	dbName := "tonotes"
	if os.Getenv("GO_ENV") == "test" {
		dbName = "tonotes_test"
	}

	// Debug logging
	fmt.Printf("Using database: %s\n", dbName)

	userRepo := repository.GetUserRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	// Create user
	err := userService.CreateUser(c, &user)
	if err != nil {
		fmt.Printf("CreateUser error: %v\n", err)
		if err.Error() == "username already exists" {
			utils.Conflict(c, "username already exists")
			return
		}
		utils.BadRequest(c, "invalid request")
		return
	}

	// Generate access token
	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		fmt.Printf("Token generation error: %v\n", err)
		utils.InternalError(c, "failed to generate token")
		return
	}

	// Generate refresh token
	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		fmt.Printf("Refresh token generation error: %v\n", err)
		utils.InternalError(c, "failed to generate refresh token")
		return
	}

	utils.Created(c, gin.H{
		"message": "user registered successfully",
		"token":   token,
		"refresh": refreshToken,
	})
}
