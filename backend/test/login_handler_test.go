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
	// Initialize MongoDB client
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	utils.MongoClient = client

	// Clear test database before starting
	if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/login", handler.LoginHandler)

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		setupMockDB   func(t *testing.T, userRepo *repository.UsersRepo)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
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
			},
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

				if _, hasToken := data["token"]; !hasToken {
					t.Error("Response missing token")
				}
				if _, hasRefresh := data["refresh"]; !hasRefresh {
					t.Error("Response missing refresh token")
				}
				if msg, ok := data["message"].(string); !ok || msg != "Login successful" {
					t.Errorf("Expected message 'Login successful', got %q", msg)
				}
			},
		},
		{
			name: "Invalid Password",
			inputJSON: `{
				"username": "testuser",
				"password": "Test12!!##"
			}`,
			expectedCode: http.StatusUnauthorized,
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
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Incorrect Password" {
					t.Errorf("Expected error 'Incorrect Password', got %q", response.Error)
				}
			},
		},
		{
			name: "User Not Found",
			inputJSON: `{
				"username": "nonexistent",
				"password": "Test12!!@@"
			}`,
			expectedCode: http.StatusUnauthorized,
			setupMockDB:  func(t *testing.T, userRepo *repository.UsersRepo) {},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid username" {
					t.Errorf("Expected error 'Invalid username', got %q", response.Error)
				}
			},
		},
		{
			name: "Invalid JSON",
			inputJSON: `{
				"username": "testuser",
				"password":
			}`,
			expectedCode: http.StatusBadRequest,
			setupMockDB:  func(t *testing.T, userRepo *repository.UsersRepo) {},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid Request" {
					t.Errorf("Expected error 'Invalid Request', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collection before each test
			if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear test database: %v", err)
			}

			// Setup mock database
			userRepo := repository.GetUsersRepo(utils.MongoClient)
			tt.setupMockDB(t, userRepo)

			// Create request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

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
