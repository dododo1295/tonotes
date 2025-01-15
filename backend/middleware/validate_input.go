package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ValidateAuthInput() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,password"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			c.Abort()
			return
		}

		c.Next()
	}
}
