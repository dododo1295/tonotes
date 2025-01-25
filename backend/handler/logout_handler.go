package handler

import (
	"main/model"
	"main/repository"
	"main/services"
	"strings"

	"main/utils"

	"github.com/gin-gonic/gin"
)

func LogoutHandler(c *gin.Context, sessionRepo *repository.SessionRepo) {
	// Get the access token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		utils.Unauthorized(c, "Invalid access token")
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Get refresh token
	refreshToken := c.GetHeader("Refresh-Token")
	if refreshToken == "" {
		utils.BadRequest(c, "Missing refresh token")
		return
	}

	// Blacklist tokens first
	if err := services.BlacklistTokens(accessToken, refreshToken); err != nil {
		utils.InternalError(c, "Failed to blacklist tokens")
		return
	}

	// Update session after successful token blacklisting
	if session, exists := c.Get("session"); exists {
		currentSession := session.(*model.Session)
		currentSession.IsActive = false
		if err := sessionRepo.UpdateSession(currentSession); err != nil {
			utils.InternalError(c, "Failed to end session")
			return
		}
	}

	utils.Success(c, gin.H{
		"message": "Successfully logged out",
	})
}
