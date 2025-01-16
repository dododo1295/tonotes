package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		//		allowedOrigins := []string{
		//			"http://localhost:3000", // Development
		//			"http://localhost:5173", // Vite default
		//			"http://127.0.0.1:3000", // Alternative localhost
		//		}
		//
		//		origin := c.Request.Header.Get("Origin")
		//		allowed := false
		//		for _, allowedOrigin := range allowedOrigins {
		//			if origin == allowedOrigin {
		//				allowed = true
		//				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		//				break
		//			}
		//		}
		//
		//		if !allowed {
		//			c.AbortWithStatus(http.StatusForbidden)
		//			return
		//		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods",
			"POST, GET, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
