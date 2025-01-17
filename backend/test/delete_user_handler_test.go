package test

import (
	"context"
	"encoding/json"
	"main/handler"
	"main/model"
	"main/services"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestDeleteUserHandler(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

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

	collection := client.Database("tonotes_test").Collection("users")
	_, err = collection.InsertOne(context.Background(), testUser)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	defer collection.DeleteMany(context.Background(), bson.M{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		token, _ := services.GenerateToken(testUserID)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		c.Set("user_id", testUserID) // Changed from "userID" to "user_id"
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

				if msg, ok := data["message"].(string); !ok || msg != "User deleted successfully" { // Changed expected message
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
