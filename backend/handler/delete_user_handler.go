package handler

import (
	"log"
	"main/repository"
	"main/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func DeleteUserHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)

	// End all sessions for the user
	if err := sessionRepo.DeleteUserSessions(userID.(string)); err != nil {
		log.Printf("Error ending user sessions: %v", err)
		utils.InternalError(c, "Failed to end all user sessions")
		return
	}

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

	// Clear session cookie
	c.SetCookie("session_id", "", -1, "/", "", true, true)

	log.Printf("User deleted successfully: %s", userID)

	c.Status(http.StatusNoContent) // return 204 on success
}
