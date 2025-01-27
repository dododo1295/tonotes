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
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting up two factor test environment")
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
	os.Setenv("USERS_COLLECTION", "users")
}

func TestTwoFactorHandler(t *testing.T) {
	// Connect to test database
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	// Database cleanup
	if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear users collection: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Setup routes
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

	tests := []struct {
		name          string
		method        string
		path          string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UserRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "Generate 2FA Secret - Success",
			method:       "GET",
			path:         "/2fa/generate",
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "Test123!@#",
					TwoFactorEnabled: false,
					CreatedAt:        time.Now(),
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
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
				if _, ok := data["qr_code"].(string); !ok {
					t.Error("Response missing QR code")
				}
			},
		},
		{
			name:   "Enable 2FA - Success",
			method: "POST",
			path:   "/2fa/enable",
			inputJSON: func() string {
				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				code, err := totp.GenerateCode(secret, time.Now())
				if err != nil {
					t.Fatalf("Failed to generate TOTP code: %v", err)
				}
				return fmt.Sprintf(`{
                    "secret": "%s",
                    "code": "%s"
                }`, secret, code)
			}(),
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				user := &model.User{
					UserID:           "test-user-id",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         "Test123!@#",
					TwoFactorEnabled: false,
					CreatedAt:        time.Now(),
				}
				_, err := userRepo.AddUser(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Response missing data object")
				}

				if msg, ok := data["message"].(string); !ok || msg != "2FA enabled successfully" {
					t.Error("Unexpected response message")
				}

				if codes, ok := data["recovery_codes"].([]interface{}); !ok || len(codes) == 0 {
					t.Error("Response missing recovery codes")
				}
			},
		},
		// Add more test cases as needed...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collections before each test
			if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear users collection: %v", err)
			}

			userRepo := repository.GetUserRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			var req *http.Request
			if tt.inputJSON != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.inputJSON))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
				t.Logf("Response body: %s", w.Body.String())
			}

			tt.checkResponse(t, w)
		})
	}
}
