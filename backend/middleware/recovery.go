package middleware

import "github.com/gin-gonic/gin"

func EnhancedRecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                // Log error
                // Cleanup resources
                // Notify monitoring service
                c.AbortWithStatus(500)
            }
        }()
        c.Next()
    }
}
