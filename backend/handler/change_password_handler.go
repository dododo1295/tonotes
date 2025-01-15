package handler

import (
	"log"
	"main/repository"
	"main/utils"
	"net/http"
	"os"
	"time"

	"main/services"

	"github.com/gin-gonic/gin"
)

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Initialize repository
	userRepo := repository.UsersRepo{
		MongoCollection: utils.MongoClient.Database(os.Getenv("MONGO_DB")).Collection("users"),
	}

	// Find user
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching user %s: %v", userID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify old password
	if !services.ComparePasswords(user.Password, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Validate new password format
	if !utils.ValidatePassword(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password does not meet requirements"})
		return
	}

	// Check if new password is same as current
	if services.ComparePasswords(user.Password, req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password cannot be the same as current password"})
		return
	}

	// Check rate limit (2 weeks)
	twoWeeks := 14 * 24 * time.Hour
	if time.Since(user.LastPasswordChange) < twoWeeks {
		nextAllowedChange := user.LastPasswordChange.Add(twoWeeks)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":               "Password can only be changed every 2 weeks",
			"next_allowed_change": nextAllowedChange,
		})
		return
	}

	// Hash new password
	hashedPassword, err := services.HashPassword(req.NewPassword)
	if err != nil {
		log.Printf("Error hashing password for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process new password"})
		return
	}

	// Update password
	result, err := userRepo.UpdateUserPassword(userID.(string), hashedPassword)
	if err != nil {
		log.Printf("Failed to update password for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	if result == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	log.Printf("Password changed successfully for user %s", userID)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
