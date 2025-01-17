package test

import (
	"encoding/json"
	"main/handler"
	"main/services"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
}

func TestLogoutHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/logout", handler.LogoutHandler)

	tests := []struct {
		name          string
		setupToken    func() (string, string)
		expectedCode  int
		expectedError string
	}{
		{
			name: "Successful Logout",
			setupToken: func() (string, string) {
				accessToken, _ := services.GenerateToken("test-user-id")
				refreshToken, _ := services.GenerateRefreshToken("test-user-id")
				return accessToken, refreshToken
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "Missing Token",
			setupToken: func() (string, string) {
				return "", ""
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid access token", // Updated message
		},
		{
			name: "Invalid Token Format",
			setupToken: func() (string, string) {
				return "invalid-token", "refresh-token"
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid access token", // Updated message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/logout", nil)

			accessToken, refreshToken := tt.setupToken()
			if accessToken != "" {
				req.Header.Set("Authorization", "Bearer "+accessToken)
			}
			if refreshToken != "" {
				req.Header.Set("Refresh-Token", refreshToken)
			}

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if tt.expectedError != "" {
				if errMsg, ok := response["error"].(string); !ok || errMsg != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errMsg)
				}
			} else {
				if msg, ok := response["message"].(string); !ok || msg != "Successfully logged out" {
					t.Errorf("Expected message 'Successfully logged out', got %q", msg)
				}
			}
		})
	}
}
