package handler

import (
	"main/repository"
	"main/utils"

	"github.com/gin-gonic/gin"
)


func GetActiveSessions(c *gin.Context, sessionRepo *repository.SessionRepo) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	sessions, err := sessionRepo.GetUserActiveSessions(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch sessions")
		return
	}

	utils.Success(c, gin.H{
		"sessions": sessions,
	})
}

func LogoutAllSessions(c *gin.Context, sessionRepo *repository.SessionRepo) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	// End all sessions for the user
	if err := sessionRepo.EndAllUserSessions(userID.(string)); err != nil {
		utils.InternalError(c, "Failed to end all sessions")
		return
	}

	// Clear current session cookie
	c.SetCookie("session_id", "", -1, "/", "", true, true)

	utils.Success(c, gin.H{
		"message": "Successfully logged out of all sessions",
	})
}
