package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"main/handler"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	fmt.Println("Setting GO_ENV=test in init")
	os.Setenv("GO_ENV", "test")
	os.Setenv("JWT_SECRET_KEY", "test_secret_key")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")

	// Register custom password validator
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
			password := fl.Field().String()
			// Password must:
			// - Be at least 8 characters
			// - Contain at least one uppercase letter
			// - Contain at least one lowercase letter
			// - Contain at least one number
			// - Contain at least one special character
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
		})
	}
}

func TestRegistrationHandler(t *testing.T) {
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")

	testClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer testClient.Disconnect(context.Background())

	utils.MongoClient = testClient

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/register", handler.RegistrationHandler)

	tests := []struct {
		name          string
		inputJSON     string
		setupFunc     func() error
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Successful Registration",
			inputJSON: `{"username":"testuser1234","password":"Test12!!@@","email":"test@example.com"}`,
			setupFunc: func() error {
				return testClient.Database("tonotes_test").Collection("users").Drop(context.Background())
			},
			expectedCode: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
				if msg, ok := data["message"].(string); !ok || msg != "user registered successfully" {
					t.Errorf("Expected message 'user registered successfully', got %q", msg)
				}
			},
		},
		{
			name:      "Duplicate Username",
			inputJSON: `{"username":"existinguser","password":"Test12!!@@","email":"another@example.com"}`,
			setupFunc: func() error {
				if err := testClient.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
					return err
				}
				_, err := testClient.Database("tonotes_test").Collection("users").InsertOne(
					context.Background(),
					bson.M{
						"username":   "existinguser",
						"password":   "$2a$10$XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
						"email":      "existing@test.com",
						"created_at": time.Now(),
						"user_id":    uuid.New().String(),
					},
				)
				return err
			},
			expectedCode: http.StatusConflict,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
			if tt.setupFunc != nil {
				if err := tt.setupFunc(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			t.Logf("Test: %s", tt.name)
			t.Logf("Making request with body: %s", tt.inputJSON)

			count, _ := testClient.Database("tonotes_test").Collection("users").CountDocuments(context.Background(), bson.M{})
			t.Logf("Documents in collection before request: %d", count)

			router.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			count, _ = testClient.Database("tonotes_test").Collection("users").CountDocuments(context.Background(), bson.M{})
			t.Logf("Documents in collection after request: %d", count)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
