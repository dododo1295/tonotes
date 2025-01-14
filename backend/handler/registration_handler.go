package handler

import (
	"main/model"
	"main/utils"
	"net/http"

	"main/services"
	"main/usecase"

	"github.com/gin-gonic/gin"
)

func RegistrationHandler(c *gin.Context) {
	var user model.User

	// Bind JSON and validate request (this will now handle email validation)
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Create user service
	userService := &usecase.UserService{
		MongoCollection: utils.MongoClient.Database("tonotes").Collection("users"),
	}

	// Create user s UUID generation and password hashing)
	userService.CreateUser(c)
	if c.IsAborted() {
		return
	}

	// getting user ID from response
	createdUserID := c.GetString("user_id")

	// Generate access token using existing service
	token, err := services.GenerateToken(createdUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Return success response with token
	c.JSON(http.StatusCreated, gin.H{
		"message": "user registered successfully",
		"token":   token,
	})
}
