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
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
    fmt.Println("Setting up change email test environment")
    os.Setenv("GO_ENV", "test")
    os.Setenv("JWT_SECRET_KEY", "test_secret_key")
    os.Setenv("JWT_EXPIRATION_TIME", "3600")
    os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
    os.Setenv("MONGO_DB", "tonotes_test")
    os.Setenv("USERS_COLLECTION", "users")
}

func TestChangeEmailHandler(t *testing.T) {
    // Initialize MongoDB client
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    if err != nil {
        t.Fatalf("Failed to connect to MongoDB: %v", err)
    }
    defer client.Disconnect(context.Background())

    utils.MongoClient = client

    // Create test user
    testUserID := uuid.New().String()
    testUsername := "testuser"
    initialEmail := "initial@example.com"
    pastTime := time.Now().Add(-24 * time.Hour * 15) // 15 days ago

    testUser := model.User{
        UserID:          testUserID,
        Username:        testUsername,
        Email:           initialEmail,
        Password:        "hashedpassword",
        CreatedAt:       time.Now(),
        LastEmailChange: pastTime,
    }

    // Set up test database
    collection := client.Database("tonotes_test").Collection("users")
    _, err = collection.InsertOne(context.Background(), testUser)
    if err != nil {
        t.Fatalf("Failed to insert test user: %v", err)
    }
    defer collection.DeleteMany(context.Background(), bson.M{})

    // Set up Gin router
    gin.SetMode(gin.TestMode)
    router := gin.New()
    router.Use(func(c *gin.Context) {
        // Add auth token setup
        token, _ := services.GenerateToken(testUserID)
        c.Request.Header.Set("Authorization", "Bearer "+token)
        c.Set("user_id", testUserID)
        c.Set("username", testUsername)
        c.Set("users_repo", &repository.UsersRepo{MongoCollection: collection})
    })
    router.POST("/change-email", handler.ChangeEmailHandler)

    tests := []struct {
        name           string
        requestBody    map[string]string
        setupFunc      func()
        expectedStatus int
        expectedBody   map[string]string
    }{
        {
            name: "Success - Valid email change",
            requestBody: map[string]string{
                "new_email": "newemail@example.com",
            },
            expectedStatus: http.StatusOK,
            expectedBody: map[string]string{
                "message": "Email updated successfully",
            },
        },
        {
            name: "Failure - Same email",
            requestBody: map[string]string{
                "new_email": initialEmail,
            },
            expectedStatus: http.StatusBadRequest,
            expectedBody: map[string]string{
                "error": "New email is same as current email",
            },
        },
        {
            name: "Failure - Rate limit",
            requestBody: map[string]string{
                "new_email": "another@example.com",
            },
            setupFunc: func() {
                _, err := collection.UpdateOne(
                    context.Background(),
                    bson.M{"username": testUsername},
                    bson.M{"$set": bson.M{"lastEmailChange": time.Now()}},
                )
                if err != nil {
                    t.Fatalf("Failed to update LastEmailChange: %v", err)
                }
            },
            expectedStatus: http.StatusTooManyRequests,
            expectedBody: map[string]string{
                "error": "Email can only be changed every 2 weeks",
            },
        },
        {
            name: "Failure - Invalid email format",
            requestBody: map[string]string{
                "new_email": "invalid-email",
            },
            expectedStatus: http.StatusBadRequest,
            expectedBody: map[string]string{
                "error": "Invalid email format",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Reset user state before each test
            _, err := collection.UpdateOne(
                context.Background(),
                bson.M{"username": testUsername},
                bson.M{"$set": bson.M{
                    "email":           initialEmail,
                    "lastEmailChange": pastTime,
                }},
            )
            if err != nil {
                t.Fatalf("Failed to reset user state: %v", err)
            }

            if tt.setupFunc != nil {
                tt.setupFunc()
            }

            // Convert request body to JSON
            jsonBody, err := json.Marshal(tt.requestBody)
            if err != nil {
                t.Fatalf("Failed to marshal request body: %v", err)
            }

            // Create request
            req, _ := http.NewRequest("POST", "/change-email", bytes.NewBuffer(jsonBody))
            req.Header.Set("Content-Type", "application/json")

            // Create response recorder
            w := httptest.NewRecorder()

            // Serve request
            router.ServeHTTP(w, req)

            // Check status code
            if w.Code != tt.expectedStatus {
                t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
            }

            // Parse response body
            var response map[string]string
            if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
                t.Fatalf("Failed to unmarshal response body: %v", err)
            }

            // Check response message
            if tt.expectedBody != nil {
                for key, expectedValue := range tt.expectedBody {
                    if actualValue, exists := response[key]; !exists || actualValue != expectedValue {
                        t.Errorf("Expected response body to contain %s: %s, got %s", key, expectedValue, actualValue)
                    }
                }
            }
        })
    }
}
