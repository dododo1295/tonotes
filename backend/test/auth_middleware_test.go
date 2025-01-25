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

	// set up redis
	setupTestRedis(t)
	defer services.TokenBlacklist.Close()

	tests := []struct {
		name           string
		setupAuth      func() string
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Valid Token",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return "Bearer " + token
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Success case doesn't return a body
				if w.Body.Len() != 0 {
					t.Error("Expected empty response body for successful auth")
				}
			},
		},
		{
			name: "No Token",
			setupAuth: func() string {
				return ""
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if errMsg, ok := response["error"].(string); !ok || errMsg != "Missing or invalid token" {
					t.Errorf("Expected 'Missing or invalid token' error, got %v", errMsg)
				}
			},
		},
		{
			name: "Invalid Token Format",
			setupAuth: func() string {
				return "Bearer invalid-token"
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if errMsg, ok := response["error"].(string); !ok || errMsg != "Invalid token" {
					t.Errorf("Expected 'Invalid token' error, got %v", errMsg)
				}
			},
		},
		{
			name: "Blacklisted Token",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				services.BlacklistTokens(token, "") // Blacklist the token
				return "Bearer " + token
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if errMsg, ok := response["error"].(string); !ok || errMsg != "Token has been invalidated" {
					t.Errorf("Expected 'Token has been invalidated' error, got %v", errMsg)
				}
			},
		},
		{
			name: "Missing Bearer Prefix",
			setupAuth: func() string {
				token, _ := services.GenerateToken("test-user-id")
				return token
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if errMsg, ok := response["error"].(string); !ok || errMsg != "Missing or invalid token" {
					t.Errorf("Expected 'Missing or invalid token' error, got %v", errMsg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.AuthMiddleware())

			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			if auth := tt.setupAuth(); auth != "" {
				req.Header.Set("Authorization", auth)
			}

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
