package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"main/handler"
	"main/model"
	"main/repository"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting GO_ENV=test in init")
	os.Setenv("GO_ENV", "test")

	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("JWT_EXPIRATION_TIME", "3600")
	os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			return len(fl.Field().String()) >= 6
		})
	}
}

func TestLoginHandler(t *testing.T) {
	fmt.Println("Starting TestLoginHandler")
	t.Log("Test starting...")

	fmt.Printf("GO_ENV value: %s\n", os.Getenv("GO_ENV"))

	envVars := map[string]string{
		"MONGO_URI":                     "mongodb://localhost:27017",
		"JWT_SECRET_KEY":                "test_secret_key",
		"JWT_EXPIRATION_TIME":           "3600",
		"REFRESH_TOKEN_EXPIRATION_TIME": "604800",
		"MONGO_DB":                      "tonotes_test",
		"USERS_COLLECTION":              "users",
	}

	for key, value := range envVars {
		t.Logf("Setting %s=%s", key, value)
		os.Setenv(key, value)
	}

	testClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer testClient.Disconnect(context.Background())

	utils.MongoClient = testClient

	if err := testClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/login", handler.LoginHandler)

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		expectedError string
		setupMockDB   func(t *testing.T, userRepo *repository.UsersRepo)
	}{
		{
			name: "Successful Login",
			inputJSON: `{
				"username": "testuser",
				"password": "Test12!!@@"
			}`,
			expectedCode: http.StatusOK,
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				hashedPass, err := services.HashPassword("Test12!!@@")
				if err != nil {
					t.Fatalf("Failed to hash password: %v", err)
				}

				testUser := model.User{
					UserID:    "test-uuid",
					Username:  "testuser",
					Password:  hashedPass,
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}

				_, err = userRepo.AddUser(context.Background(), &testUser)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}

				storedUser, err := userRepo.FindUserByUsername("testuser")
				if err != nil {
					t.Fatalf("Failed to retrieve test user: %v", err)
				}

				if hashedPass != storedUser.Password {
					t.Fatalf("Password mismatch after storage:\nOriginal: %s\nStored: %s",
						hashedPass, storedUser.Password)
				}
			},
		},
		{
			name: "Invalid Password",
			inputJSON: `{
				"username": "testuser",
				"password": "Test12!!##"
			}`,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Incorrect Password",
			setupMockDB: func(t *testing.T, userRepo *repository.UsersRepo) {
				hashedPass, _ := services.HashPassword("Test12!!@@")
				testUser := model.User{
					UserID:    "test-uuid",
					Username:  "testuser",
					Password:  hashedPass,
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}
				_, err := userRepo.AddUser(context.Background(), &testUser)
				if err != nil {
					t.Fatalf("Failed to insert test user: %v", err)
				}
			},
		},
		{
			name: "User Not Found",
			inputJSON: `{
				"username": "nonexistent",
				"password": "Test12!!@@"
			}`,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid username",
			setupMockDB:   func(t *testing.T, userRepo *repository.UsersRepo) {},
		},
		{
			name: "Invalid JSON",
			inputJSON: `{
				"username": "testuser",
				"password":
			}`,
			expectedCode:  http.StatusBadRequest,
			expectedError: "Invalid Request",
			setupMockDB:   func(t *testing.T, userRepo *repository.UsersRepo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := utils.MongoClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear test database: %v", err)
			}

			userRepo := repository.GetUsersRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.expectedError != "" {
				if errMsg, ok := response["error"].(string); !ok || errMsg != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errMsg)
				}
			} else {
				if _, hasToken := response["token"]; !hasToken {
					t.Error("Response missing token")
				}
				if _, hasRefresh := response["refresh"]; !hasRefresh {
					t.Error("Response missing refresh token")
				}
				if msg, ok := response["message"].(string); !ok || msg != "Login successful" {
					t.Errorf("Expected message 'Login successful', got %q", msg)
				}
			}
		})
	}
}
