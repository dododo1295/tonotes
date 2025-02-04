package test

import (
	"context"
	"encoding/json"
	"fmt"
	"main/handler"
	"main/model"
	"main/repository"
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
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	testutils.SetupTestEnvironment()

	if utils.JWTSecretKey == "" {
		panic("JWT_SECRET_KEY environment variable not set")
	}
}

func setupTokenBlacklist(t *testing.T) {
	if utils.JWTSecretKey == "" {
		t.Fatal("JWT secret key not set")
	}
	t.Logf("Using JWT secret key (length: %d)", len(utils.JWTSecretKey))

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL environment variable not set")
	}

	var err error
	services.TokenBlacklist, err = services.NewTokenBlacklist(redisURL)
	if err != nil {
		t.Fatalf("Failed to initialize token blacklist: %v", err)
	}

	// Test Redis connection and clear existing data
	ctx := context.Background()
	if err := services.TokenBlacklist.Client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	if err := services.TokenBlacklist.Client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to clear Redis: %v", err)
	}

	// Create test tokens with future expiration
	claims := jwt.MapClaims{
		"exp":     time.Now().Add(24 * time.Hour).Unix(), // Set expiration to 24 hours in the future
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
		"user_id": "test-user",
	}

	// Create access token
	testAccessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessTokenString, err := testAccessToken.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create test access token: %v", err)
	}

	// Create refresh token (add refresh type to claims)
	claims["type"] = "refresh"
	testRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshTokenString, err := testRefreshToken.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create test refresh token: %v", err)
	}

	t.Logf("Testing blacklist with tokens:")
	t.Logf("Access Token: %s", accessTokenString)
	t.Logf("Refresh Token: %s", refreshTokenString)

	// Try to blacklist the tokens
	err = services.BlacklistTokens(accessTokenString, refreshTokenString)
	if err != nil {
		t.Fatalf("BlacklistTokens failed: %v", err)
	}

	// Verify the tokens were blacklisted
	accessKey := fmt.Sprintf("blacklist:access:%s", accessTokenString)
	refreshKey := fmt.Sprintf("blacklist:refresh:%s", refreshTokenString)

	// List all keys in Redis
	keys, err := services.TokenBlacklist.Client.Keys(ctx, "blacklist:*").Result()
	if err != nil {
		t.Fatalf("Failed to list Redis keys: %v", err)
	}
	t.Logf("Redis keys after blacklisting: %v", keys)

	// Check if tokens exist in Redis
	accessExists, err := services.TokenBlacklist.Client.Exists(ctx, accessKey).Result()
	if err != nil {
		t.Fatalf("Failed to check access token: %v", err)
	}
	refreshExists, err := services.TokenBlacklist.Client.Exists(ctx, refreshKey).Result()
	if err != nil {
		t.Fatalf("Failed to check refresh token: %v", err)
	}

	t.Logf("Access token exists: %v", accessExists > 0)
	t.Logf("Refresh token exists: %v", refreshExists > 0)

	if accessExists == 0 || refreshExists == 0 {
		t.Fatal("Initial blacklist functionality test failed - tokens not stored in Redis")
	}

	// Clear the test data
	if err := services.TokenBlacklist.Client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to clear test data: %v", err)
	}

	t.Log("Successfully initialized and tested Redis blacklist")
}

func clearRedisBlacklist(t *testing.T) {
	if services.TokenBlacklist != nil && services.TokenBlacklist.Client != nil {
		ctx := context.Background()
		if err := services.TokenBlacklist.Client.FlushDB(ctx).Err(); err != nil {
			t.Logf("Warning: Failed to clear Redis blacklist: %v", err)
		}
	}
}

func TestLogoutHandler(t *testing.T) {

	if utils.JWTSecretKey == "" {
		t.Fatal("JWT_SECRET_KEY environment variable not set")
	}
	t.Logf("JWT secret key length: %d", len(utils.JWTSecretKey))
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Setup Redis token blacklist
	setupTokenBlacklist(t)

	// Verify Redis is working with direct operation
	ctx := context.Background()
	testKey := "test:key"
	err := services.TokenBlacklist.Client.Set(ctx, testKey, "true", time.Hour).Err()
	if err != nil {
		t.Fatalf("Redis direct operation failed: %v", err)
	}
	t.Log("Direct Redis operation successful")
	services.TokenBlacklist.Client.Del(ctx, testKey)

	// Ensure Redis cleanup after tests
	defer func() {
		clearRedisBlacklist(t)
		if services.TokenBlacklist != nil {
			services.TokenBlacklist.Close()
		}
	}()

	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Set up database and collections
	db := client.Database(os.Getenv("MONGO_DB_TEST"))
	sessionsCollection := db.Collection("sessions")
	sessionRepo := &repository.SessionRepo{
		MongoCollection: sessionsCollection,
	}

	// Setup Gin router
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		setupAuth     func(t *testing.T) (string, string, *model.Session)
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *model.Session, string, string)
		setupDatabase func(*testing.T) *model.Session
	}{
		{
			name: "Successful Logout",
			setupDatabase: func(t *testing.T) *model.Session {
				userID := "test-user-id"
				session := &model.Session{
					SessionID:      "test-session-id",
					UserID:         userID,
					CreatedAt:      time.Now(),
					ExpiresAt:      time.Now().Add(24 * time.Hour),
					LastActivityAt: time.Now(),
					DeviceInfo:     "test device",
					IPAddress:      "127.0.0.1",
					IsActive:       true,
				}

				_, err := sessionsCollection.InsertOne(context.Background(), session)
				if err != nil {
					t.Fatalf("Failed to create test session: %v", err)
				}
				return session
			},
			setupAuth: func(t *testing.T) (string, string, *model.Session) {
				userID := "test-user-id"

				// Create tokens with future expiration
				claims := jwt.MapClaims{
					"exp":     time.Now().Add(24 * time.Hour).Unix(),
					"iat":     time.Now().Unix(),
					"iss":     "toNotes",
					"user_id": userID,
				}

				accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				accessTokenString, err := accessToken.SignedString([]byte(utils.JWTSecretKey))
				if err != nil {
					t.Fatalf("Failed to create access token: %v", err)
				}

				claims["type"] = "refresh"
				refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				refreshTokenString, err := refreshToken.SignedString([]byte(utils.JWTSecretKey))
				if err != nil {
					t.Fatalf("Failed to create refresh token: %v", err)
				}

				return accessTokenString, refreshTokenString, &model.Session{
					SessionID:      "test-session-id",
					UserID:         userID,
					IsActive:       true,
					LastActivityAt: time.Now(),
				}
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, session *model.Session, accessToken, refreshToken string) {
				ctx := context.Background()

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if msg, ok := data["message"].(string); !ok || msg != "Successfully logged out" {
					t.Errorf("Expected message 'Successfully logged out', got %v", msg)
				}

				// Get the raw token (without Bearer prefix)
				rawAccessToken := accessToken // Already raw from setupAuth

				// Check blacklist status
				accessKey := fmt.Sprintf("blacklist:access:%s", rawAccessToken)
				refreshKey := fmt.Sprintf("blacklist:refresh:%s", refreshToken)

				t.Logf("Checking for blacklist keys:")
				t.Logf("Access key: %s", accessKey)
				t.Logf("Refresh key: %s", refreshKey)

				time.Sleep(100 * time.Millisecond) // Give Redis time to process

				// List all keys in Redis
				keys, _ := services.TokenBlacklist.Client.Keys(ctx, "blacklist:*").Result()
				t.Logf("All Redis keys: %v", keys)

				accessExists, _ := services.TokenBlacklist.Client.Exists(ctx, accessKey).Result()
				refreshExists, _ := services.TokenBlacklist.Client.Exists(ctx, refreshKey).Result()

				if accessExists == 0 {
					t.Error("Access token not blacklisted")
					// Try manual blacklisting for debugging
					err := services.BlacklistTokens(rawAccessToken, refreshToken)
					if err != nil {
						t.Logf("Manual blacklisting failed: %v", err)
					}
				}
				if refreshExists == 0 {
					t.Error("Refresh token not blacklisted")
				}
			},
		},
		{
			name: "Missing Authorization Token",
			setupAuth: func(t *testing.T) (string, string, *model.Session) {
				return "", "", nil
			},
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, _ *model.Session, _, _ string) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if response["error"] != "Invalid access token" {
					t.Errorf("Expected error 'Invalid access token', got %v", response["error"])
				}
			},
			setupDatabase: func(t *testing.T) *model.Session {
				return nil
			},
		},
		{
			name: "Missing Refresh Token",
			setupAuth: func(t *testing.T) (string, string, *model.Session) {
				userID := "test-user-id"
				accessToken, _ := services.GenerateToken(userID)
				return accessToken, "", nil
			},
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, _ *model.Session, _, _ string) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if response["error"] != "Missing refresh token" {
					t.Errorf("Expected error 'Missing refresh token', got %v", response["error"])
				}
			},
			setupDatabase: func(t *testing.T) *model.Session {
				return nil
			},
		},
		{
			name: "Session Not Found",
			setupAuth: func(t *testing.T) (string, string, *model.Session) {
				accessToken, refreshToken := createTestTokens(t, "nonexistent-user-id")

				t.Logf("Generated tokens for non-existent user:")
				t.Logf("Access Token: %s", accessToken)
				t.Logf("Refresh Token: %s", refreshToken)

				return accessToken, refreshToken, nil
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, _ *model.Session, accessToken, refreshToken string) {
				ctx := context.Background()

				// First verify we can parse these tokens
				token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
					return []byte(utils.JWTSecretKey), nil
				})
				if err != nil {
					t.Fatalf("Failed to parse access token: %v", err)
				}
				if !token.Valid {
					t.Fatal("Access token is not valid")
				}

				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Logf("Full response: %+v", response)
					t.Fatal("Response missing data object")
				}

				if msg, ok := data["message"].(string); !ok || msg != "Successfully logged out" {
					t.Errorf("Expected message 'Successfully logged out', got %v", msg)
				}

				// Give Redis time to process
				time.Sleep(100 * time.Millisecond)

				// Check blacklist status
				accessKey := fmt.Sprintf("blacklist:access:%s", accessToken)
				refreshKey := fmt.Sprintf("blacklist:refresh:%s", refreshToken)

				keys, _ := services.TokenBlacklist.Client.Keys(ctx, "blacklist:*").Result()
				t.Logf("All Redis keys: %v", keys)

				accessExists, _ := services.TokenBlacklist.Client.Exists(ctx, accessKey).Result()
				refreshExists, _ := services.TokenBlacklist.Client.Exists(ctx, refreshKey).Result()

				if accessExists == 0 {
					t.Error("Access token not blacklisted")
				}
				if refreshExists == 0 {
					t.Error("Refresh token not blacklisted")
				}
			},
			setupDatabase: func(t *testing.T) *model.Session {
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear Redis before each test
			clearRedisBlacklist(t)

			// Setup test data
			session := tt.setupDatabase(t)

			// Setup router for this test
			router := gin.New()
			router.Use(func(c *gin.Context) {
				// Log the incoming request headers
				t.Logf("Request headers:")
				t.Logf("Authorization: %s", c.GetHeader("Authorization"))
				t.Logf("Refresh-Token: %s", c.GetHeader("Refresh-Token"))
				c.Next()
			})

			router.POST("/logout", func(c *gin.Context) {
				if session != nil {
					c.Set("session", session)
				}
				handler.LogoutHandler(c, sessionRepo)
			})

			// Setup request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/logout", nil)

			// Add authentication if provided
			accessToken, refreshToken, _ := tt.setupAuth(t)
			if accessToken != "" {
				req.Header.Set("Authorization", "Bearer "+accessToken)
				t.Logf("Set Authorization header: Bearer %s", accessToken)
			}
			if refreshToken != "" {
				req.Header.Set("Refresh-Token", refreshToken)
				t.Logf("Set Refresh-Token header: %s", refreshToken)
			}

			// Execute request
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Check response and blacklist
			tt.checkResponse(t, w, session, accessToken, refreshToken)

			// Verify session state in database if it exists
			if session != nil {
				var updatedSession model.Session
				err := sessionsCollection.FindOne(context.Background(),
					bson.M{"session_id": session.SessionID}).Decode(&updatedSession)
				if err != nil {
					t.Fatalf("Failed to find session: %v", err)
				}

				if updatedSession.IsActive {
					t.Error("Session should be marked as inactive")
				}
			}
		})
	}
}

func verifyTokenBlacklist(t *testing.T, accessToken, refreshToken string) {
	if services.TokenBlacklist == nil || services.TokenBlacklist.Client == nil {
		t.Fatal("Redis client is not initialized")
	}

	ctx := context.Background()

	// Add small delay to ensure Redis operations complete
	time.Sleep(100 * time.Millisecond)

	// Debug logging
	keys, err := services.TokenBlacklist.Client.Keys(ctx, "blacklist:*").Result()
	if err != nil {
		t.Logf("Failed to list Redis keys: %v", err)
	} else {
		t.Logf("Current Redis keys: %v", keys)
	}

	// Check access token
	accessKey := fmt.Sprintf("blacklist:access:%s", accessToken)
	accessExists, err := services.TokenBlacklist.Client.Exists(ctx, accessKey).Result()
	if err != nil {
		t.Errorf("Failed to check access token blacklist: %v", err)
	}
	if accessExists == 0 {
		t.Errorf("Access token not found in blacklist (key: %s)", accessKey)
	}

	// Check refresh token
	refreshKey := fmt.Sprintf("blacklist:refresh:%s", refreshToken)
	refreshExists, err := services.TokenBlacklist.Client.Exists(ctx, refreshKey).Result()
	if err != nil {
		t.Errorf("Failed to check refresh token blacklist: %v", err)
	}
	if refreshExists == 0 {
		t.Errorf("Refresh token not found in blacklist (key: %s)", refreshKey)
	}
}

func createTestTokens(t *testing.T, userID string) (string, string) {
	// Use a fixed future time for testing
	futureTime := time.Now().Add(24 * time.Hour)

	// Create base claims
	claims := jwt.MapClaims{
		"exp":     futureTime.Unix(),
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
		"user_id": userID,
	}

	// Create access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessTokenString, err := accessToken.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create access token: %v", err)
	}

	// Create refresh token
	refreshClaims := jwt.MapClaims{
		"exp":     futureTime.Unix(),
		"iat":     time.Now().Unix(),
		"iss":     "toNotes",
		"type":    "refresh",
		"user_id": userID,
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(utils.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create refresh token: %v", err)
	}

	return accessTokenString, refreshTokenString
}
