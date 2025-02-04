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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/pquerna/otp/totp"
)

func init() {
	testutils.SetupTestEnvironment()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			return len(fl.Field().String()) >= 6
		})
	}
}

func TestLoginHandler(t *testing.T) {
	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	// Set MongoDB client
	utils.MongoClient = client
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Initialize collections
	collections := []string{"users", "sessions"}
	for _, collName := range collections {
		if err := db.Collection(collName).Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
		}
		if err := db.CreateCollection(context.Background(), collName); err != nil {
			if !strings.Contains(err.Error(), "NamespaceExists") {
				t.Fatalf("Failed to create collection %s: %v", collName, err)
			}
		}
	}

	// Initialize session repository
	sessionRepo := repository.GetSessionRepo(client)
	sessionRepo.MongoCollection = db.Collection("sessions")

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/login", func(c *gin.Context) {
		handler.LoginHandler(c, sessionRepo)
	})

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UserRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder, *repository.SessionRepo)
	}{
		{
			name: "Successful login - No 2FA",
			inputJSON: `{
                "username": "test@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := db.Collection("users")
				hashedPassword, _ := services.HashPassword("Test123!@#")

				user := &model.User{
					UserID:           "test-uuid",
					Username:         "test@example.com",
					Email:            "test@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: false,
				}

				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
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
			name: "Invalid Credentials",
			inputJSON: `{
                "username": "wrong@example.com",
                "password": "WrongPass123"
            }`,
			expectedCode: http.StatusUnauthorized,
			setupMockDB:  func(t *testing.T, userRepo *repository.UserRepo) {},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
				var response utils.Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				expectedError := "Invalid username"
				if response.Error != "Invalid username" {
					t.Errorf("Expected '%s' error, got: %s", expectedError, response.Error)
				}
			},
		},
		{
			name: "2FA Required",
			inputJSON: `{
                "username": "test2fa@example.com",
                "password": "Test123!@#"
            }`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := db.Collection("users")
				hashedPassword, _ := services.HashPassword("Test123!@#")

				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  base32.StdEncoding.EncodeToString([]byte("TESTSECRET123")),
				}

				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
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
			},
		},
		{
			name: "2FA Success",
			inputJSON: func() string {
				secret := base32.StdEncoding.EncodeToString([]byte("TESTSECRET123"))
				validCode, _ := totp.GenerateCode(secret, time.Now())
				return fmt.Sprintf(`{
                    "username": "test2fa@example.com",
                    "password": "Test123!@#",
                    "two_factor_code": "%s"
                }`, validCode)
			}(),
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UserRepo) {
				collection := db.Collection("users")
				hashedPassword, _ := services.HashPassword("Test123!@#")

				user := &model.User{
					UserID:           "test-2fa-uuid",
					Username:         "test2fa@example.com",
					Email:            "test2fa@example.com",
					Password:         hashedPassword,
					CreatedAt:        time.Now(),
					TwoFactorEnabled: true,
					TwoFactorSecret:  base32.StdEncoding.EncodeToString([]byte("TESTSECRET123")),
				}

				_, err := collection.InsertOne(context.Background(), user)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, sessionRepo *repository.SessionRepo) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collections before each test
			for _, collName := range collections {
				if err := db.Collection(collName).Drop(context.Background()); err != nil {
					t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
				}
			}

			userRepo := repository.GetUserRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
				t.Logf("Response body: %s", w.Body.String())
			}

			tt.checkResponse(t, w, sessionRepo)
		})
	}
}
