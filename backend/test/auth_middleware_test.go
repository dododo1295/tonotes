package test

import (
	"encoding/json"
	"main/middleware"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	utils.InitJWT()

	tests := []struct {
		name          string
		setupAuth     func() string
		expectedCode  int
		expectedError string
	}{
		{
			name: "Valid Token",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return "Bearer " + token
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "No Token",
			setupAuth: func() string {
				return ""
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid token",
		},
		{
			name: "Invalid Token Format",
			setupAuth: func() string {
				return "Bearer invalid-token"
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid token",
		},
		{
			name: "Blacklisted Token",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				services.BlacklistTokens(token, "") // Blacklist the token
				return "Bearer " + token
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Token has been invalidated",
		},
		{
			name: "Missing Bearer Prefix",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router with test endpoint
			router := gin.New()
			router.Use(middleware.AuthMiddleware())

			// Add a test endpoint that returns 200 if middleware passes
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			// Add auth header if provided
			if auth := tt.setupAuth(); auth != "" {
				req.Header.Set("Authorization", auth)
			}

			// Serve request
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// For error cases, verify the error message
			if tt.expectedError != "" {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if errMsg, ok := response["error"].(string); !ok || errMsg != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errMsg)
				}
			}
		})
	}
}
