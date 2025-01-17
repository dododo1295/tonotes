package handler

import (
	"fmt"
	"main/model"
	"main/repository"
	"main/services"
	"main/usecase"
	"main/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func LoginHandler(c *gin.Context) {
	var loginReq model.LoginRequest

	// Bind JSON request
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	// Debug: Print the query
	fmt.Printf("Looking for user with username: %s\n", loginReq.Username)

	// Get repository and create service
	userRepo := repository.GetUsersRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	// Find user using service
	user, err := userService.FindUserByUsername(loginReq.Username)
	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username"})
		return
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username"})
		return
	}

	// Debug: Print both passwords
	fmt.Printf("Found user password: %s\n", user.Password)
	fmt.Printf("Provided password: %s\n", loginReq.Password)

	// Check password
	checkPassword, err := services.VerifyPassword(user.Password, loginReq.Password)
	if err != nil {
		fmt.Printf("Password verification error: %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect Password"})
		return
	}
	if !checkPassword {
		fmt.Printf("Password verification failed\n")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect Password"})
		return
	}

	// Generate access token
	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate refresh token
	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"refresh": refreshToken,
	})
}
