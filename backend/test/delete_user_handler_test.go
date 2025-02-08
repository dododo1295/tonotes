package test

import (
	"context"
	"encoding/json"
	"log"
	"main/handler"
	"main/model"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	err := godotenv.Load("../.env") // Adjust path if needed
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func TestDeleteUserHandler(t *testing.T) {
	// Get MongoDB connection details from environment variables
	mongoURI := os.Getenv("TEST_MONGO_URI")
	if mongoURI == "" {
		t.Fatal("TEST_MONGO_URI environment variable not set")
	}
	mongoDBName := os.Getenv("TEST_MONGO_DB")
	if mongoDBName == "" {
		t.Fatal("TEST_MONGO_DB environment variable not set")
	}
	usersCollectionName := "users" // Define the users collection name (or get it from an env var if needed)

	// Connect to MongoDB using the connection string from the environment
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Set utils.MongoClient for the handler to use
	utils.MongoClient = client

	testUserID := uuid.New().String()
	testUsername := "testuser"
	testEmail := "test@example.com"

	testUser := model.User{
		UserID:    testUserID,
		Username:  testUsername,
		Email:     testEmail,
		Password:  "hashedpassword",
		CreatedAt: time.Now(),
	}

	// *** IMPORTANT: Set environment variables for test ***
	os.Setenv("MONGO_DB", mongoDBName)
	os.Setenv("USERS_COLLECTION", usersCollectionName)

	collection := client.Database(mongoDBName).Collection(usersCollectionName)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		token, _ := services.GenerateToken(testUserID)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		c.Set("user_id", testUserID)
		c.Set("username", testUsername)
	})
	router.DELETE("/delete-user", handler.DeleteUserHandler)

	tests := []struct {
		name           string
		setupFunc      func()
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "Success - User deleted",
			expectedStatus: http.StatusOK,
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

				if msg, ok := data["message"].(string); !ok || msg != "User deleted successfully" {
					t.Errorf("Expected success message, got %v", msg)
				}
			},
		},
		{
			name: "Failure - User not found",
			setupFunc: func() {
				collection.DeleteOne(context.Background(), bson.M{"user_id": testUserID})
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "User not found" {
					t.Errorf("Expected 'User not found' error, got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the database before each test
			collection.DeleteMany(context.Background(), bson.M{})

			// Re-insert the test user for each test.
			_, err = collection.InsertOne(context.Background(), testUser)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			req, _ := http.NewRequest("DELETE", "/delete-user", nil)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Log response for debugging
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
