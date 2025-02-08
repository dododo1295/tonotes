package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/handler"
	"main/model"
	"main/repository"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	testClient *mongo.Client
	testDBName string
)

func init() {
	fmt.Println("Setting up two factor test environment")
	if err := godotenv.Load("../.env"); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}

	testDBName = os.Getenv("TEST_MONGO_DB")
	if testDBName == "" {
		panic("TEST_MONGO_DB environment variable not set")
	}
	fmt.Printf("Using test database: %s\n", testDBName)
}

func setupTestDB(t *testing.T) (*mongo.Client, func()) {
	t.Logf("Setting up test database: %s", testDBName)

	mongoURI := os.Getenv("TEST_MONGO_URI")
	if mongoURI == "" {
		t.Fatal("TEST_MONGO_URI environment variable not set")
	}

	// Set test environment
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", testDBName)

	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Verify connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}
	t.Log("Successfully connected to MongoDB")

	// Return cleanup function
	cleanup := func() {
		t.Log("Cleaning up test database")
		if err := client.Database(testDBName).Collection("users").Drop(context.Background()); err != nil {
			t.Logf("Failed to cleanup test database: %v", err)
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Logf("Failed to disconnect from test database: %v", err)
		}
	}

	return client, cleanup
}

func setupTest2FARouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	router.GET("/2fa/generate", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.Generate2FASecretHandler(c)
	})
	router.POST("/2fa/enable", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.Enable2FAHandler(c)
	})
	router.POST("/2fa/verify", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.Verify2FAHandler(c)
	})
	router.POST("/2fa/disable", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.Disable2FAHandler(c)
	})
	router.POST("/2fa/recovery", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.UseRecoveryCodeHandler(c)
	})

	return router
}

func cleanupDatabase(t *testing.T, client *mongo.Client) {
	t.Log("Cleaning up users collection")
	if err := client.Database(testDBName).Collection("users").Drop(context.Background()); err != nil {
		t.Logf("Warning: Failed to clear users collection: %v", err)
	}
}

func verifyUserInDatabase(t *testing.T, client *mongo.Client, userID string) *model.User {
	collection := client.Database(testDBName).Collection("users")
	var user model.User
	err := collection.FindOne(context.Background(), bson.M{"user_id": userID}).Decode(&user)
	if err != nil {
		t.Logf("Failed to find user %s: %v", userID, err)
		return nil
	}
	t.Logf("Found user: %+v", user)
	return &user
}

func TestTwoFactorHandlers(t *testing.T) {
	// Setup test database
	client, cleanup := setupTestDB(t)
	if client == nil {
		return // Skip tests if DB setup failed
	}
	defer cleanup()

	// Set MongoDB client
	utils.MongoClient = client

	// Setup Gin router
	router := setupTest2FARouter()

	userRepo := repository.GetUserRepo(utils.MongoClient) // Get UserRepo instance here

	tests := []struct {
		name          string
		endpoint      string
		method        string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UserRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *repository.UserRepo)
	}{
		{
			name:         "Generate 2FA Secret - Success",
			endpoint:     "/2fa/generate",
			method:       "GET", // Corrected method
			inputJSON:    ``,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if _, ok := data["secret"].(string); !ok {
					t.Error("Response missing secret")
				}

				qrCode, ok := data["qr_code"].(string)
				if !ok {
					t.Error("Response missing qr_code")
				}

				if !strings.HasPrefix(qrCode, "data:image/png;base64,") {
					t.Error("QR code should be a base64 encoded PNG")
				}
				message, ok := data["message"].(string) // Extract message from data
				if !ok || message != "Successfully generated 2FA" {
					t.Errorf("Expected 'Successfully generated 2FA' message, got: %v", message)
				}
			},
		},
		{
			name:         "Generate 2FA Secret - 2FA Already Enabled",
			endpoint:     "/2fa/generate",
			method:       "GET", // Corrected method
			inputJSON:    ``,
			expectedCode: http.StatusBadRequest,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "2FA is already enabled" {
					t.Errorf("Expected '2FA is already enabled' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Enable 2FA - Invalid Code",
			endpoint:     "/2fa/enable",
			method:       "POST",
			inputJSON:    `{"secret": "JBSWY3DPEHPKDDQ", "code": "INVALID_CODE"}`,
			expectedCode: http.StatusBadRequest,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "Invalid 2FA code" {
					t.Errorf("Expected 'Invalid 2FA code' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Verify 2FA - Success",
			endpoint:     "/2fa/verify",
			method:       "POST",
			expectedCode: http.StatusOK,
			inputJSON: func() string {
				secret := "JBSWY3DPEHPKDDQ"
				validCode, _ := totp.GenerateCode(secret, time.Now())
				return fmt.Sprintf(`{"code": "%s"}`, validCode)
			}(),
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")

				// This is the proper approach now
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,              // Already Enabled
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ", // Secret is set
				}

				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				message, ok := data["message"].(string) // Extract message from data
				if !ok || message != "2FA code valid" {
					t.Errorf("Expected '2FA code valid' message, got: %v", message)
				}
			},
		},
		{
			name:         "Verify 2FA - Invalid Code",
			endpoint:     "/2fa/verify",
			method:       "POST",
			inputJSON:    `{"code": "INVALID_CODE"}`,
			expectedCode: http.StatusUnauthorized,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ",
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "Invalid 2FA code" {
					t.Errorf("Expected 'Invalid 2FA code' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Verify 2FA - 2FA Not Enabled",
			endpoint:     "/2fa/verify",
			method:       "POST",
			inputJSON:    `{"code": "123456"}`,
			expectedCode: http.StatusBadRequest,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "2FA is not enabled" {
					t.Errorf("Expected '2FA is not enabled' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Disable 2FA - Success",
			endpoint:     "/2fa/disable",
			method:       "POST",
			expectedCode: http.StatusOK,
			inputJSON: func() string {
				secret := "JBSWY3DPEHPKDDQ"
				validCode, _ := totp.GenerateCode(secret, time.Now())
				return fmt.Sprintf(`{"code": "%s"}`, validCode)
			}(),
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")

				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,              // Already Enabled
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ", // Secret is set
				}

				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				message, ok := data["message"].(string) // Extract message from data
				if !ok || message != "2FA disabled successfully" {
					t.Errorf("Expected '2FA disabled successfully' message, got: %v", message)
				}

				user, err := userRepo.FindUser("test-user-id")
				if err != nil {
					t.Fatalf("Failed to find user: %v", err)
				}
				if user.TwoFactorEnabled {
					t.Error("2FA should be disabled in the database")
				}
			},
		},
		{
			name:         "Disable 2FA - Invalid Code",
			endpoint:     "/2fa/disable",
			method:       "POST",
			inputJSON:    `{"code": "INVALID_CODE"}`,
			expectedCode: http.StatusUnauthorized,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ",
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "Invalid 2FA code" {
					t.Errorf("Expected 'Invalid 2FA code' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Disable 2FA - 2FA Not Enabled",
			endpoint:     "/2fa/disable",
			method:       "POST",
			inputJSON:    `{"code": "123456"}`,
			expectedCode: http.StatusBadRequest,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "2FA is not enabled" {
					t.Errorf("Expected '2FA is not enabled' error, got: %s", response.Error)
				}
			},
		},
		{
			name:         "Use Recovery Code - Success",
			endpoint:     "/2fa/recovery",
			method:       "POST",
			expectedCode: http.StatusOK,
			inputJSON:    `{"recovery_code": "AAAA-BBBB-CCCC-DDDD"}`,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")

				// Hash the recovery code and add it to the user's RecoveryCodes
				recoveryCode := strings.ToUpper(strings.ReplaceAll("AAAA-BBBB-CCCC-DDDD", "-", ""))
				hashedCode := utils.HashString(recoveryCode)

				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ",
					RecoveryCodes:    []string{hashedCode, "another-hashed-code"}, // Set a valid code
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				message, ok := data["message"].(string) // Extract message from data
				if !ok || message != "Recovery code accepted" {
					t.Errorf("Expected 'Recovery code accepted' message, got: %v", message)
				}

				remainingCodes, ok := data["remaining_codes"].(float64)
				if !ok || int(remainingCodes) != 1 {
					t.Errorf("Expected 1 remaining code, got: %v", remainingCodes)
				}

				user, err := userRepo.FindUser("test-user-id")
				if err != nil {
					t.Fatalf("Failed to find user: %v", err)
				}
				if len(user.RecoveryCodes) != 1 {
					t.Errorf("Expected 1 recovery code remaining, got: %d", len(user.RecoveryCodes))
				}
				recoveryCode := strings.ToUpper(strings.ReplaceAll("AAAA-BBBB-CCCC-DDDD", "-", ""))
				hashedCode := utils.HashString(recoveryCode)
				if user.RecoveryCodes[0] == hashedCode {
					t.Error("The used recovery code should have been removed from the database")
				}
			},
		},
		{
			name:         "Use Recovery Code - Invalid Code",
			endpoint:     "/2fa/recovery",
			method:       "POST",
			inputJSON:    `{"recovery_code": "INVALID-CODE"}`,
			expectedCode: http.StatusUnauthorized,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := client.Database(testDBName).Collection("users")
				// Ensure the user is 2FA enabled and has some (but not the invalid) recovery codes.
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "hashedpassword",
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  "JBSWY3DPEHPKDDQ",
					RecoveryCodes:    []string{"valid-hashed-code"}, // Set a valid, *different* code
				}
				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, userRepo *repository.UserRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response.Error != "Invalid recovery code" {
					t.Errorf("Expected 'Invalid recovery code' error, got: %s", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the setup mock db
			cleanupDatabase(t, client)
			tt.setupMockDB(t, userRepo)

			// Perform request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.endpoint, bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// Check the status code
			if w.Code != tt.expectedCode {
				t.Errorf("Test: %s, Expected status code %d, got %d", tt.name, tt.expectedCode, w.Code)
				t.Logf("Response body: %s", w.Body.String()) // Log response body for debugging
			}

			// Call the check response
			tt.checkResponse(t, w, userRepo)
		})
	}
}
