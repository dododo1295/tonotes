package test

import (
	"bytes"
	"context"
	"fmt"
	"main/handler"
	"main/model"
	"main/repository"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
    fmt.Println("Setting GO_ENV=test in init")
    os.Setenv("GO_ENV", "test")
    os.Setenv("JWT_SECRET_KEY", "test_secret_key")
    os.Setenv("JWT_EXPIRATION_TIME", "3600")
    os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
    os.Setenv("MONGO_DB", "tonotes_test")
    os.Setenv("SESSION_COLLECTION", "sessions")

    if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
        v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
            return len(fl.Field().String()) >= 6
        })
    }
}

func TestLoginHandler(t *testing.T) {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    if err != nil {
        t.Fatalf("Failed to connect to MongoDB: %v", err)
    }
    defer client.Disconnect(context.Background())

    utils.MongoClient = client

    // Clear test databases before starting
    if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
        t.Fatalf("Failed to clear users collection: %v", err)
    }
    if err := client.Database("tonotes_test").Collection("sessions").Drop(context.Background()); err != nil {
        t.Fatalf("Failed to clear sessions collection: %v", err)
    }

    // Initialize session repository
    sessionRepo := repository.GetSessionRepo(client)

    // Set up Gin router
    gin.SetMode(gin.TestMode)
    router := gin.Default()

    // Update router to include session repo
    router.POST("/login", func(c *gin.Context) {
        handler.LoginHandler(c, sessionRepo)
    })

    tests := []struct {
        name          string
        inputJSON     string
        expectedCode  int
        setupMockDB   func(t *testing.T, userRepo *repository.UsersRepo)
        checkResponse func(*testing.T, *httptest.ResponseRecorder)
    }{
        // [Previous test cases remain the same]
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Clear collections before each test
            if err := client.Database("tonotes_test").Collection("users").Drop(context.Background()); err != nil {
                t.Fatalf("Failed to clear users collection: %v", err)
            }
            if err := client.Database("tonotes_test").Collection("sessions").Drop(context.Background()); err != nil {
                t.Fatalf("Failed to clear sessions collection: %v", err)
            }

            userRepo := repository.GetUsersRepo(utils.MongoClient)
            tt.setupMockDB(t, userRepo)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(tt.inputJSON))
            req.Header.Set("Content-Type", "application/json")

            router.ServeHTTP(w, req)

            t.Logf("Response Status: %d", w.Code)
            t.Logf("Response Body: %s", w.Body.String())

            if w.Code != tt.expectedCode {
                t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
            }

            tt.checkResponse(t, w)

            // For successful login, verify session was created
            if tt.expectedCode == http.StatusOK {
                var sessions []*model.Session
                cursor, err := sessionRepo.MongoCollection.Find(context.Background(), bson.M{"user_id": "test-uuid"})
                if err != nil {
                    t.Errorf("Failed to query sessions: %v", err)
                }
                if err := cursor.All(context.Background(), &sessions); err != nil {
                    t.Errorf("Failed to decode sessions: %v", err)
                }
                if len(sessions) == 0 {
                    t.Error("No session was created for successful login")
                }
            }
        })
    }
}
