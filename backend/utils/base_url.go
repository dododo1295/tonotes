package utils

import "github.com/gin-gonic/gin"

// Helper function for getting baseURL of project
func GetBaseURL(c *gin.Context) string {
	return c.Request.URL.Scheme + "://" + c.Request.Host + "/api/v1"
}
