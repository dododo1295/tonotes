package handler

import (
	"log"
	"main/repository"
	"main/usecase"
	"main/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func ChangePasswordHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	// First, get the user to check last password change time
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching user %s: %v", userID, err)
		utils.InternalError(c, "Failed to fetch user details")
		return
	}

	// Check rate limiting before attempting password change
	twoWeeks := 14 * 24 * time.Hour
	if time.Since(user.LastPasswordChange) < twoWeeks {
		nextAllowedChange := user.LastPasswordChange.Add(twoWeeks)
		utils.TooManyRequests(c, "Password can only be changed every 2 weeks", gin.H{
			"next_allowed_change": nextAllowedChange,
		})
		return
	}

	err = userService.UpdateUserPassword(userID.(string), req.OldPassword, req.NewPassword)
	if err != nil {
		switch err.Error() {
		case "user not found":
			utils.NotFound(c, "User not found")
		case "current password incorrect":
			utils.Unauthorized(c, "Current password is incorrect")
		case "password does not meet requirements":
			utils.BadRequest(c, "New password does not meet requirements")
		case "new password same as current":
			utils.BadRequest(c, "New password cannot be the same as current password")
		default:
			log.Printf("Error updating password for user %s: %v", userID, err)
			utils.InternalError(c, "Failed to update password")
		}
		return
	}

	log.Printf("Password changed successfully for user %s", userID)
	utils.Success(c, gin.H{"message": "Password updated successfully"})
}
