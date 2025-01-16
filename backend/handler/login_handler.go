package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"main/model"
	"main/services"
	"main/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

// LoginHandler handles user login requests
func LoginHandler(c *gin.Context) {
	var loginReq model.LoginRequest

	// bind to struct
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	// get user data from database
	dbName := "tonotes"
	if os.Getenv("GO_ENV") == "test" {
		dbName = "tonotes_test"
	}

	// Debug: Print the query
	fmt.Printf("Looking for user with username: %s in database: %s\n", loginReq.Username, dbName)

	// Find user directly using MongoDB client
	var fetchUser model.User
	err := utils.MongoClient.Database(dbName).Collection("users").
		FindOne(context.Background(), bson.M{"username": loginReq.Username}).
		Decode(&fetchUser)

	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username"})
		return
	}

	// Debug: Print both passwords
	fmt.Printf("Found user password: %s\n", fetchUser.Password)
	fmt.Printf("Provided password: %s\n", loginReq.Password)

	// check password
	checkPassword, err := services.VerifyPassword(fetchUser.Password, loginReq.Password)
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
	token, err := services.GenerateToken(fetchUser.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate refresh token
	refreshToken, err := services.GenerateRefreshToken(fetchUser.UserID)
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
