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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	// Set minimal required env vars for JWT
	os.Setenv("JWT_SECRET_KEY", "test_secret")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	utils.InitJWT()
	gin.SetMode(gin.TestMode)
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Valid Token",
			authHeader:     createValidToken(t),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No Token",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing or invalid token",
		},
		{
			name:           "Invalid Token Format",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid token",
		},
		{
			name:           "Missing Bearer Prefix",
			authHeader:     createValidToken(t)[7:], // Remove "Bearer " prefix
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing or invalid token",
		},
		{
			name:           "Malformed Bearer Token",
			authHeader:     "Bearer abc def",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid token",
		},
		{
			name:           "Expired Token",
			authHeader:     createExpiredToken(t),
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid token",
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
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedError != "" {
				var response struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Error != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, response.Error)
				}
			} else if w.Body.Len() != 0 {
				t.Error("Expected empty response body for successful auth")
			}
		})
	}
}

// Helper functions
func createValidToken(t *testing.T) string {
	token, err := services.GenerateToken("test-user-id")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	return "Bearer " + token
}

func createExpiredToken(t *testing.T) string {
	// Create token that expired 1 hour ago
	expiredTime := time.Now().Add(-1 * time.Hour)
	claims := map[string]interface{}{
		"user_id": "test-user-id",
		"exp":     expiredTime.Unix(),
		"iat":     expiredTime.Unix(),
		"iss":     "toNotes",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
	signedToken, err := token.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to generate expired token: %v", err)
	}
	return "Bearer " + signedToken
}
