package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"main/handler"
	"main/model"
	"main/repository"
	"main/test/testutils"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	testutils.SetupTestEnvironment()

	// Register custom password validator
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", validatePassword)
	}
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	if len(password) < 6 {
		return false
	}

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

func setupRegistrationTest(t *testing.T) (*repository.UserRepo, func()) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)

	// Set MongoDB client
	utils.MongoClient = client

	// Get database reference
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Create users collection
	err := db.CreateCollection(context.Background(), "users")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Logf("Warning: Failed to create users collection: %v", err)
	}

	// Initialize repository
	userRepo := repository.GetUserRepo(client)
	userRepo.MongoCollection = db.Collection("users")

	// Return cleanup function
	return userRepo, func() {
		t.Log("Running registration test cleanup")
		if err := db.Collection("users").Drop(context.Background()); err != nil {
			t.Logf("Warning: Failed to drop users collection: %v", err)
		}
		cleanup()
	}
}

func TestRegistrationHandler(t *testing.T) {
	// Setup
	userRepo, cleanup := setupRegistrationTest(t)
	defer cleanup()

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Add logging middleware
		t.Logf("Processing request: %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	})
	router.POST("/register", handler.RegistrationHandler)

	tests := []struct {
		name          string
		inputJSON     string
		setupTestData func(*testing.T) error
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
		checkDatabase func(*testing.T) error
	}{
		{
			name:      "Successful Registration",
			inputJSON: `{"username":"testuser1234","password":"Test12!!@@","email":"test@example.com"}`,
			setupTestData: func(t *testing.T) error {
				return userRepo.MongoCollection.Drop(context.Background())
			},
			expectedCode: http.StatusCreated,
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

				// Check required fields
				requiredFields := []string{"token", "refresh", "message"}
				for _, field := range requiredFields {
					if _, exists := data[field]; !exists {
						t.Errorf("Response missing required field: %s", field)
					}
				}

				if msg, ok := data["message"].(string); !ok || msg != "user registered successfully" {
					t.Errorf("Expected message 'user registered successfully', got %v", msg)
				}
			},
			checkDatabase: func(t *testing.T) error {
				var user model.User
				err := userRepo.MongoCollection.FindOne(context.Background(),
					bson.M{"username": "testuser1234"}).Decode(&user)
				if err != nil {
					return err
				}
				if user.Email != "test@example.com" {
					t.Errorf("Expected email test@example.com, got %s", user.Email)
				}
				return nil
			},
		},
		{
			name:      "Duplicate Username",
			inputJSON: `{"username":"existinguser","password":"Test12!!@@","email":"another@example.com"}`,
			setupTestData: func(t *testing.T) error {
				if err := userRepo.MongoCollection.Drop(context.Background()); err != nil {
					return err
				}
				_, err := userRepo.MongoCollection.InsertOne(context.Background(), model.User{
					UserID:    uuid.New().String(),
					Username:  "existinguser",
					Password:  "$2a$10$XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
					Email:     "existing@test.com",
					CreatedAt: time.Now(),
				})
				return err
			},
			expectedCode: http.StatusConflict,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "username already exists" {
					t.Errorf("Expected error 'username already exists', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid Password - Too Short",
			inputJSON:    `{"username":"testuser1234","password":"a!1","email":"test@example.com"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid Password - No Number",
			inputJSON:    `{"username":"testuser1234","password":"abcdef!!","email":"test@example.com"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid Password - No Special Character",
			inputJSON:    `{"username":"testuser1234","password":"abcdef123","email":"test@example.com"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid Email Format",
			inputJSON:    `{"username":"testuser1234","password":"Test12!!@@","email":"invalidemail"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
		{
			name:         "Empty Username",
			inputJSON:    `{"username":"","password":"Test12!!@@","email":"test@example.com"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid JSON",
			inputJSON:    `{"username":"testuser1234","password":"Test12!!@@","email":}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "invalid request" {
					t.Errorf("Expected error 'invalid request', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data
			if tt.setupTestData != nil {
				if err := tt.setupTestData(t); err != nil {
					t.Fatalf("Failed to setup test data: %v", err)
				}
			}

			// Count documents before request
			count, err := userRepo.MongoCollection.CountDocuments(context.Background(), bson.M{})
			if err != nil {
				t.Logf("Warning: Failed to count documents: %v", err)
			}
			t.Logf("Documents before test: %d", count)

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			// Log test information
			t.Logf("Running test: %s", tt.name)
			t.Logf("Request body: %s", tt.inputJSON)

			// Execute request
			router.ServeHTTP(w, req)

			// Log response
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Check response
			tt.checkResponse(t, w)

			// Check database state if needed
			if tt.checkDatabase != nil {
				if err := tt.checkDatabase(t); err != nil {
					t.Errorf("Database check failed: %v", err)
				}
			}

			// Count documents after request
			count, err = userRepo.MongoCollection.CountDocuments(context.Background(), bson.M{})
			if err != nil {
				t.Logf("Warning: Failed to count documents: %v", err)
			}
			t.Logf("Documents after test: %d", count)
		})
	}
}
