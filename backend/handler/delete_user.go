package handler

import (
	"log"
	"main/repository"
	"main/utils"

	"github.com/gin-gonic/gin"
)

func DeleteUserHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	userRepo := repository.GetUsersRepo(utils.MongoClient)

	deletedCount, err := userRepo.DeleteUserByID(userID.(string))
	if err != nil {
		log.Printf("Failed to delete user %s: %v", userID, err)
		utils.InternalError(c, "Failed to delete user")
		return
	}

	if deletedCount == 0 {
		utils.NotFound(c, "User not found")
		return
	}

	log.Printf("User deleted successfully: %s", userID)
	utils.Success(c, gin.H{"message": "User deleted successfully"})
}
