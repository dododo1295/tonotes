package handler

import (
	"main/repository"
	"main/utils"
	"net/http"
	"os"

	"main/services"

	"github.com/gin-gonic/gin"
)

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,password"`
}

func ChangePasswordHandler(c *gin.Context) {
	// Get userID from the JWT token (set by AuthMiddleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid token"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Validate new password format first
	if !utils.ValidatePassword(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Initialize repository
	userRepo := repository.UsersRepo{
		MongoCollection: utils.MongoClient.Database(os.Getenv("MONGO_DB")).Collection("users"),
	}

	// Find user
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Verify old password matches
	match, err := services.VerifyPassword(user.Password, req.OldPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !match {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid old password"})
		return
	}

	// Hash the new password
	hashedPassword, err := services.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash new password"})
		return
	}

	// Update password with hashed version
	modifiedCount, err := userRepo.UpdateUserPassword(userID.(string), hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	if modifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "password updated successfully",
	})
}
