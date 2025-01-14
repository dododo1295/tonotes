package handler

import (
	"main/services"
	"main/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RefreshTokenHandler(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh"})
		return
	}

	refreshToken := strings.TrimPrefix(authHeader, "Bearer ")

	//validate refresh
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(utils.JWTSecretKey), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh"})
	}
	// validate token type
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" || claims["user_id"] == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		return
	}

	//expired?
	if exp, ok := claims["exp"].(float64); ok && time.Unix(int64(exp), 0).Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token has expired"})
		return
	}
	// issue new access Token
	newAccessToken, err := services.GenerateToken(claims["user_id"].(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to genereate new access token"})
	}
	//issue new refresh token
	newRefreshToken, err := services.GenerateRefreshToken(claims["user_id"].(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate new refresh token"})
		return
	}

	//respond with new tokens
	c.JSON(http.StatusOK, gin.H{
		"access_token":      newAccessToken,
		"new_refresh_token": newRefreshToken,
	})

}
