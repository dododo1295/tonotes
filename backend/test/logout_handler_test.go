package test

import (
	"encoding/json"
	"main/handler"
	"main/services"
	"main/utils"
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
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Successful Logout",
			setupToken: func() (string, string) {
				accessToken, _ := services.GenerateToken("test-user-id")
				refreshToken, _ := services.GenerateRefreshToken("test-user-id")
				return accessToken, refreshToken
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if msg, ok := data["message"].(string); !ok || msg != "Successfully logged out" {
					t.Errorf("Expected message 'Successfully logged out', got %q", msg)
				}
			},
		},
		{
			name: "Missing Token",
			setupToken: func() (string, string) {
				return "", ""
			},
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Missing or invalid access token" {
					t.Errorf("Expected error 'Missing or invalid access token', got %q", response.Error)
				}
			},
		},
		{
			name: "Invalid Token Format",
			setupToken: func() (string, string) {
				return "invalid-token", "refresh-token"
			},
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid access token" {
					t.Errorf("Expected error 'Invalid access token', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request recorder
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/logout", nil)

			// Setup tokens
			accessToken, refreshToken := tt.setupToken()
			if accessToken != "" {
				req.Header.Set("Authorization", "Bearer "+accessToken)
			}
			if refreshToken != "" {
				req.Header.Set("Refresh-Token", refreshToken)
			}

			// Serve request
			router.ServeHTTP(w, req)

			// Log response for debugging
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Run custom response checks
			tt.checkResponse(t, w)
		})
	}
}
