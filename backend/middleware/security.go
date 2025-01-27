package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		var w http.ResponseWriter = c.Writer
		c.Request.Body = http.MaxBytesReader(w, c.Request.Body, maxSize)

		if c.Request.ContentLength > maxSize {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}

		c.Next()
	}
}
