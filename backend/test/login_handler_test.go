package test

import (
	"bytes"
	"context"
	"encoding/base32"
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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/pquerna/otp/totp"
)

func init() {
	fmt.Println("Setting GO_ENV=test in init")
	// Instead of setting environment variables here, use testutils
	testutils.SetupTestEnvironment()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			return len(fl.Field().String()) >= 6
		})
	}
}

// TestLoginHandler verifies the login functionality including:
// - User authentication with valid credentials
// - Session management and creation
// - Session limit enforcement (max 5 active sessions)
// - Least active session termination when session limit is reached
// - Response format and token generation
func TestLoginHandler(t *testing.T) {
	t.Log("=== Starting LoginHandler Test Suite ===")

	// Use the SetupTestDB function that worked in users_repo_test.go
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Set the MongoDB client in utils
	utils.MongoClient = client
	t.Log("MongoDB client set in utils")

	// Create database reference
	db := client.Database("tonotes_test")

	// Ensure required collections exist
	collections := []string{"users", "sessions"}
	for _, collName := range collections {
		t.Logf("Ensuring collection exists: %s", collName)
		if err := db.CreateCollection(context.Background(), collName); err != nil {
			if !strings.Contains(err.Error(), "NamespaceExists") {
				t.Fatalf("Failed to create collection %s: %v", collName, err)
			}
			t.Logf("Collection %s already exists", collName)
		}
	}

	// Initialize session repository
	sessionRepo := repository.GetSessionRepo(client)
	t.Log("Session repository initialized")

	sessionRepo.MongoCollection = db.Collection("sessions")
	t.Log("Seesion repository collection set")

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/login", func(c *gin.Context) {
		handler.LoginHandler(c, sessionRepo)
	})
	t.Log("Router configured with login handler")

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UsersRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *repository.SessionRepo)
	}{
		{
			name: "Successful login - No 2FA",
			inputJSON: `{
                "username": "test@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: Successful login - No 2FA")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				user := &model.User{
					UserID:           "test-uuid",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created test user with ID: %v", result.InsertedID)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for successful login")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, ok := data["token"].(string); !ok {
					t.Error("Response missing token")
				}
				if _, ok := data["refresh"].(string); !ok {
					t.Error("Response missing refresh token")
				}
			},
		},
		{
			name: "2FA Required - No Code Provided",
			inputJSON: `{
                "username": "test2fa@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: 2FA Required - No Code Provided")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  "TESTSECRET123",
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created 2FA test user with ID: %v", result.InsertedID)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for 2FA required")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if requires2FA, ok := data["requires_2fa"].(bool); !ok || !requires2FA {
					t.Error("Response should indicate 2FA is required")
				}

				if _, ok := data["token"]; ok {
					t.Error("Response should not contain token when 2FA is required")
				}
			},
		},
		{
			name: "2FA Success - With Valid Code",
			inputJSON: func() string {
				t.Log("Setting up 2FA Success test case with valid code")

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				t.Logf("Generated Base32 secret: %s", secret)

				validCode, err := totp.GenerateCode(secret, time.Now())
				if err != nil {
					t.Fatalf("Failed to generate TOTP code: %v", err)
				}
				t.Logf("Generated valid TOTP code: %s", validCode)

				return fmt.Sprintf(`{
                    "username": "test2fa@example.com",
                    "password": "Test123!@#",
                    "two_factor_code": "%s"
                }`, validCode)
			}(),
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: 2FA Success - With Valid Code")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  secret,
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created 2FA test user with ID: %v", result.InsertedID)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for successful 2FA login")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error != "" {
					t.Errorf("Unexpected error in response: %s", response.Error)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, ok := data["token"].(string); !ok {
					t.Error("Response missing token")
				}
				if _, ok := data["refresh"].(string); !ok {
					t.Error("Response missing refresh token")
				}
			},
		},
		{
			name: "Login with max sessions",
			inputJSON: `{
                "username": "test@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: Login with max sessions")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				user := &model.User{
					UserID:    "test-uuid",
					Username:  "test@example.com",
					Email:     "test@example.com",
					Password:  hashedPassword,
					CreatedAt: time.Now(),
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created test user with ID: %v", result.InsertedID)

				// Create max sessions with logging
				t.Logf("Creating %d test sessions", handler.MaxActiveSessions)
				for i := 0; i < handler.MaxActiveSessions; i++ {
					session := &model.Session{
						SessionID:      fmt.Sprintf("session-%d", i),
						UserID:         "test-uuid",
						CreatedAt:      time.Now().Add(-time.Duration(i) * time.Hour),
						ExpiresAt:      time.Now().Add(24 * time.Hour),
						LastActivityAt: time.Now().Add(-time.Duration(i) * time.Hour),
						DeviceInfo:     fmt.Sprintf("device-%d", i),
						IPAddress:      "127.0.0.1",
						IsActive:       true,
					}
					result, err := sessionRepo.MongoCollection.InsertOne(context.Background(), session)
					if err != nil {
						t.Fatalf("Failed to insert test session %d: %v", i, err)
					}
					t.Logf("Created session %d with ID: %v", i, result.InsertedID)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for max sessions login")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				notice, hasNotice := data["notice"].(string)
				if !hasNotice {
					t.Error("Response missing notice about ended session")
				} else {
					t.Logf("Received notice: %s", notice)
					if notice != "Logged out of least active session due to session limit" {
						t.Errorf("Unexpected notice message: %s", notice)
					}
				}

				// Verify session counts
				t.Log("Verifying session counts")
				time.Sleep(100 * time.Millisecond)
				sessions, err := sessionRepo.GetUserActiveSessions("test-uuid")
				if err != nil {
					t.Fatalf("Failed to get active sessions: %v", err)
				}

				t.Logf("Found %d active sessions", len(sessions))
				if len(sessions) != handler.MaxActiveSessions {
					t.Errorf("Expected %d active sessions, got %d",
						handler.MaxActiveSessions, len(sessions))
				}
			},
		},
		{
			name: "2FA Success - Time-based Code Validation",
			inputJSON: func() string {
				t.Log("Setting up 2FA Success test case with time-based validation")

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				t.Logf("Generated Base32 secret: %s", secret)

				validCode, err := totp.GenerateCode(secret, time.Now())
				if err != nil {
					t.Fatalf("Failed to generate TOTP code: %v", err)
				}
				t.Logf("Generated valid TOTP code: %s for current time", validCode)

				return fmt.Sprintf(`{
                    "username": "test2fa@example.com",
                    "password": "Test123!@#",
                    "two_factor_code": "%s"
                }`, validCode)
			}(),
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: 2FA Success - Time-based Code Validation")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  secret,
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created 2FA test user with ID: %v", result.InsertedID)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for time-based 2FA login")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error != "" {
					t.Errorf("Unexpected error in response: %s", response.Error)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, ok := data["token"].(string); !ok {
					t.Error("Response missing token")
				}
				if _, ok := data["refresh"].(string); !ok {
					t.Error("Response missing refresh token")
				}
			},
		},
		{
			name: "2FA Failure - Expired Code",
			inputJSON: func() string {
				t.Log("Setting up 2FA Failure test case with expired code")

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				t.Logf("Generated Base32 secret: %s", secret)

				pastTime := time.Now().Add(-2 * time.Minute)
				expiredCode, err := totp.GenerateCode(secret, pastTime)
				if err != nil {
					t.Fatalf("Failed to generate TOTP code: %v", err)
				}
				t.Logf("Generated expired TOTP code: %s (from 2 minutes ago)", expiredCode)

				return fmt.Sprintf(`{
                    "username": "test2fa@example.com",
                    "password": "Test123!@#",
                    "two_factor_code": "%s"
                }`, expiredCode)
			}(),
			expectedCode: http.StatusUnauthorized,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				t.Log("Setting up test case: 2FA Failure - Expired Code")

				collection := utils.MongoClient.Database("tonotes_test").Collection("users")
				t.Logf("Using collection: %s", collection.Name())
				userRepo.MongoCollection = collection

				hashedPassword, err := services.HashPassword("Test123!@#")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  secret,
				}

				result, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
				t.Logf("Created 2FA test user with ID: %v", result.InsertedID)
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				t.Log("Checking response for expired 2FA code")
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error != "Invalid 2FA code" {
					t.Errorf("Expected 'Invalid 2FA code' error, got: %s", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("=== Starting test case: %s ===", tt.name)

			// Clear collections before each test
			for _, collName := range collections {
				t.Logf("Clearing collection: %s", collName)
				if err := db.Collection(collName).Drop(context.Background()); err != nil {
					t.Fatalf("Failed to clear %s collection: %v", collName, err)
				}

				// Recreate collection
				t.Logf("Recreating collection: %s", collName)
				if err := db.CreateCollection(context.Background(), collName); err != nil {
					if !strings.Contains(err.Error(), "NamespaceExists") {
						t.Fatalf("Failed to create collection %s: %v", collName, err)
					}
					t.Logf("Collection %s already exists", collName)
				}
			}

			userRepo := repository.GetUsersRepo(utils.MongoClient)
			t.Log("Setting up mock database")
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			t.Logf("Making request with body: %s", tt.inputJSON)

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
				t.Logf("Response body: %s", w.Body.String())
			}

			t.Log("Checking response")
			tt.checkResponse(t, w, sessionRepo)
			t.Logf("=== Completed test case: %s ===", tt.name)
		})
	}
}
