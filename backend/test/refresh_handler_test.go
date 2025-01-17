package test

import (
	"encoding/json"
	"fmt"
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
	fmt.Println("Setting GO_ENV=test in init")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
}

// Helper function to create Authorization header
func createAuthHeader(t *testing.T, tokenType string) string {
	switch tokenType {
	case "valid_refresh":
		token, err := services.GenerateRefreshToken("test-user-id")
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}
		t.Logf("Generated refresh token: %s", token)
		return "Bearer " + token
	case "access_token":
		token, err := services.GenerateToken("test-user-id")
		if err != nil {
			t.Fatalf("Failed to generate access token: %v", err)
		}
		return "Bearer " + token
	case "invalid":
		return "Bearer invalid-token"
	case "no_bearer":
		return "some-token"
	case "empty":
		return ""
	default:
		t.Fatalf("Unknown token type: %s", tokenType)
		return ""
	}
}

func TestRefreshTokenHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/refresh", handler.RefreshTokenHandler)

	tests := []struct {
		name          string
		tokenType     string
		expectedCode  int
		expectedError string
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "Successful Refresh",
			tokenType:    "valid_refresh",
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, hasAccessToken := data["access_token"]; !hasAccessToken {
					t.Error("Response missing access_token")
				}
				if _, hasRefreshToken := data["new_refresh_token"]; !hasRefreshToken {
					t.Error("Response missing new_refresh_token")
				}
			},
		},
		{
			name:          "Missing Authorization Header",
			tokenType:     "empty",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid refresh",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Missing or invalid refresh" {
					t.Errorf("Expected error 'Missing or invalid refresh', got %q", response.Error)
				}
			},
		},
		{
			name:          "Invalid Token Format",
			tokenType:     "invalid",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid refresh",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid refresh" {
					t.Errorf("Expected error 'invalid refresh', got %q", response.Error)
				}
			},
		},
		{
			name:          "No Bearer Prefix",
			tokenType:     "no_bearer",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Missing or invalid refresh",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Missing or invalid refresh" {
					t.Errorf("Expected error 'Missing or invalid refresh', got %q", response.Error)
				}
			},
		},
		{
			name:          "Invalid Token Type",
			tokenType:     "access_token",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid claims",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid claims" {
					t.Errorf("Expected error 'invalid claims', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/refresh", nil)

			authHeader := createAuthHeader(t, tt.tokenType)
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}

			r.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
