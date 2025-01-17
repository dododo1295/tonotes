package handler

import (
	"log"
	"main/repository"
	"main/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

func ChangeEmailHandler(c *gin.Context) {
	// Get userID from the JWT token (set by AuthMiddleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid token"})
		return
	}

	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Initialize repository
	userRepo := repository.GetUsersRepo(utils.MongoClient)

	// Get current user details
	currentUser, err := userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user details"})
		return
	}

	// Check if new email is same as current
	if currentUser.Email == req.NewEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New email is same as current email"})
		return
	}

	// Check rate limit (2 weeks)
	twoWeeks := 14 * 24 * time.Hour
	if time.Since(currentUser.LastEmailChange) < twoWeeks {
		nextAllowedChange := currentUser.LastEmailChange.Add(twoWeeks)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":               "Email can only be changed every 2 weeks",
			"next_allowed_change": nextAllowedChange,
		})
		return
	}

	// Update email
	result, err := userRepo.UpdateUserEmail(userID.(string), req.NewEmail)
	if err != nil {
		log.Printf("Failed to update email for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while updating email"})
		return
	}

	if result == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	log.Printf("Email changed successfully for user %s", userID)
	c.JSON(http.StatusOK, gin.H{"message": "Email updated successfully"})
}
