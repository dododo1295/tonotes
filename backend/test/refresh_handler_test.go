package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/services"
	"main/test/testutils"
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
	testutils.SetupTestEnvironment()
}

// Helper function to create test tokens
func createTestToken(t *testing.T, tokenType string, userID string) string {
	if utils.JWTSecretKey == "" {
		t.Fatal("JWT secret key not set")
	}

	claims := jwt.MapClaims{
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
		"user_id": userID,
	}

	if tokenType == "refresh" {
		claims["type"] = "refresh"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create %s token: %v", tokenType, err)
	}

	return tokenString
}

// Helper function to create Authorization header
func createAuthHeader(t *testing.T, tokenType string) string {
	switch tokenType {
	case "valid_refresh":
		token := createTestToken(t, "refresh", "test-user-id")
		t.Logf("Generated refresh token: %s", token)
		return "Bearer " + token
	case "access_token":
		token := createTestToken(t, "access", "test-user-id")
		t.Logf("Generated access token: %s", token)
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

func setupRefreshTest(t *testing.T) func() {
	// Initialize Redis for token blacklist
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL environment variable not set")
	}

	var err error
	services.TokenBlacklist, err = services.NewTokenBlacklist(redisURL)
	if err != nil {
		t.Fatalf("Failed to initialize token blacklist: %v", err)
	}

	// Verify Redis connection
	ctx := context.Background()
	if err := services.TokenBlacklist.Client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	return func() {
		if services.TokenBlacklist != nil {
			services.TokenBlacklist.Close()
		}
	}
}

func TestRefreshTokenHandler(t *testing.T) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Setup Redis
	cleanup := setupRefreshTest(t)
	defer cleanup()

	// Verify JWT secret is set
	if utils.JWTSecretKey == "" {
		t.Fatal("JWT secret key not set")
	}
	t.Logf("Using JWT secret key (length: %d)", len(utils.JWTSecretKey))

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		t.Logf("Processing request: %s %s", c.Request.Method, c.Request.URL.Path)
		t.Logf("Authorization header: %s", c.GetHeader("Authorization"))
		c.Next()
	})
	router.POST("/refresh", handler.RefreshTokenHandler)

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

			router.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
