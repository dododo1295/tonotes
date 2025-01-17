package handler

import (
	"main/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RefreshTokenHandler(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh"})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh"})
		return
	}

	refreshToken := parts[1]

	userID, err := services.ValidateRefreshToken(refreshToken)
	if err != nil {
		switch err.Error() {
		case "invalid token type":
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		case "token has expired":
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token has expired"})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh"})
		}
		return
	}

	newAccessToken, err := services.GenerateToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	newRefreshToken, err := services.GenerateRefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":      newAccessToken,
		"new_refresh_token": newRefreshToken,
	})
}
