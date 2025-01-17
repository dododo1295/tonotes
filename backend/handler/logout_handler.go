package handler

import (
	"main/model"
	"main/repository"
	"main/services"
	"strings"

	"fmt"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func LogoutHandler(c *gin.Context, sessionRepo *repository.SessionRepo) {
	// Get the access token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		utils.Unauthorized(c, "Missing or invalid access token")
		return
	}

	// Extract access token without "Bearer " prefix
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate the access token format
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil {
		utils.Unauthorized(c, "Invalid access token")
		return
	}

	// Get and validate the refresh token
	refreshToken := c.GetHeader("Refresh-Token")
	if refreshToken == "" {
		utils.BadRequest(c, "Missing refresh token")
		return
	}

	// End the current session
	if session, exists := c.Get("session"); exists {
		currentSession := session.(*model.Session)
		currentSession.IsActive = false
		err := sessionRepo.UpdateSession(currentSession)
		if err != nil {
			utils.InternalError(c, "Failed to end session")
			return
		}
	}

	// Blacklist both tokens
	if err := services.BlacklistTokens(accessToken, refreshToken); err != nil {
		utils.InternalError(c, "Failed to blacklist tokens")
		return
	}

	utils.Success(c, gin.H{
		"message": "Successfully logged out",
	})
}
