package handler

import (
	"main/services"
	"main/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

func RefreshTokenHandler(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		utils.Unauthorized(c, "Missing or invalid refresh")
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		utils.Unauthorized(c, "Missing or invalid refresh")
		return
	}

	refreshToken := parts[1]

	userID, err := services.ValidateRefreshToken(refreshToken)
	if err != nil {
		switch err.Error() {
		case "invalid token type":
			utils.Unauthorized(c, "invalid claims")
		case "token has expired":
			utils.Unauthorized(c, "refresh token has expired")
		default:
			utils.Unauthorized(c, "invalid refresh")
		}
		return
	}

	newAccessToken, err := services.GenerateToken(userID)
	if err != nil {
		utils.InternalError(c, "Failed to generate access token")
		return
	}

	newRefreshToken, err := services.GenerateRefreshToken(userID)
	if err != nil {
		utils.InternalError(c, "Failed to generate refresh token")
		return
	}

	utils.Success(c, gin.H{
		"access_token":      newAccessToken,
		"new_refresh_token": newRefreshToken,
	})
}
