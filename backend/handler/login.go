package handler

import (
	"fmt"
	"main/model"
	"main/repository"
	"main/services"
	"main/usecase"
	"main/utils"

	"github.com/gin-gonic/gin"
)

func LoginHandler(c *gin.Context) {
	var loginReq model.LoginRequest

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		utils.BadRequest(c, "Invalid Request") // Changed this line
		return
	}

	fmt.Printf("Looking for user with username: %s\n", loginReq.Username)

	userRepo := repository.GetUsersRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	user, err := userService.FindUserByUsername(loginReq.Username)
	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		utils.Unauthorized(c, "Invalid username")
		return
	}

	if user == nil {
		utils.Unauthorized(c, "Invalid username")
		return
	}

	fmt.Printf("Found user password: %s\n", user.Password)
	fmt.Printf("Provided password: %s\n", loginReq.Password)

	checkPassword, err := services.VerifyPassword(user.Password, loginReq.Password)
	if err != nil {
		fmt.Printf("Password verification error: %v\n", err)
		utils.Unauthorized(c, "Incorrect Password")
		return
	}
	if !checkPassword {
		fmt.Printf("Password verification failed\n")
		utils.Unauthorized(c, "Incorrect Password")
		return
	}

	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		utils.InternalError(c, "Failed to generate token")
		return
	}

	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		utils.InternalError(c, "Failed to generate refresh token")
		return
	}

	utils.Success(c, gin.H{
		"message": "Login successful",
		"token":   token,
		"refresh": refreshToken,
	})
}
