package handler

import (
	"log"
	"main/repository"
	"main/usecase"
	"main/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func ChangePasswordHandler(c *gin.Context) {
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

	userRepo := repository.GetUsersRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	err := userService.UpdateUserPassword(userID.(string), req.OldPassword, req.NewPassword)
	if err != nil {
		// Map errors to appropriate HTTP responses
		switch err.Error() {
		case "user not found":
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case "current password incorrect":
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		case "password does not meet requirements":
			c.JSON(http.StatusBadRequest, gin.H{"error": "New password does not meet requirements"})
		case "new password same as current":
			c.JSON(http.StatusBadRequest, gin.H{"error": "New password cannot be the same as current"})
		default:
			log.Printf("Error updating password for user %s: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		}
		return
	}

	log.Printf("Password changed successfully for user %s", userID)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
