package middleware

import "github.com/gin-gonic/gin"

func CacheControlMiddleware(duration string) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Cache-Control", "public, max-age="+duration)
        c.Next()
    }
}
