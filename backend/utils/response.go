package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Status  int         `json:"-"`                 // HTTP status code
	Message string      `json:"message,omitempty"` // Optional message
	Error   string      `json:"error,omitempty"`   // Error message
	Data    interface{} `json:"data,omitempty"`    // Response data
}

func NewResponse() *Response {
	return &Response{
		Status: http.StatusOK,
	}
}

// Success responses
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, &Response{
		Status: http.StatusOK,
		Data:   data,
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, &Response{
		Status:  http.StatusCreated,
		Message: "Resource created successfully",
		Data:    data,
	})
}

// Error responses
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, &Response{
		Status: http.StatusUnauthorized,
		Error:  message,
	})
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, &Response{
		Status: http.StatusBadRequest,
		Error:  message,
	})
}

func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, &Response{
		Status: http.StatusNotFound,
		Error:  message,
	})
}

func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, &Response{
		Status: http.StatusInternalServerError,
		Error:  message,
	})
}

func TooManyRequests(c *gin.Context, message string, data ...interface{}) {
	response := &Response{
		Status: http.StatusTooManyRequests,
		Error:  message,
	}
	if len(data) > 0 {
		response.Data = data[0]
	}
	c.JSON(http.StatusTooManyRequests, response)
}

func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, &Response{
		Status: http.StatusConflict,
		Error:  message,
	})
}

// Forbidden response
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, &Response{
		Status: http.StatusForbidden,
		Error:  message,
	})
}
