package test

import (
	"bytes"
	"context"
	"encoding/json"
	"main/handler"
	"main/repository"
	"main/usecase"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")
}

func TestCreateNoteHandler(t *testing.T) {
	// Set up environment
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("MONGO_DB", "tonotes_test")

	// Connect to test database
	testClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer testClient.Disconnect(context.Background())

	utils.MongoClient = testClient

	// Drop test collection before starting
	if err := testClient.Database("tonotes_test").Collection("notes").Drop(context.Background()); err != nil {
		t.Fatalf("Failed to clear test collection: %v", err)
	}

	// Initialize repository and service
	notesRepo := repository.GetNotesRepo(testClient)
	notesService := &usecase.NotesService{
		NotesRepo: notesRepo,
	}

	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/notes", func(c *gin.Context) {
		c.Set("userID", "test-user") // Simulate auth middleware
		handler.CreateNoteHandler(c, notesService)
	})

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
		setupTestData func() error
	}{
		{
			name: "Successful Creation",
			inputJSON: `{
				"title": "Test Note",
				"content": "Test Content",
				"tags": ["test"]
			}`,
			expectedCode: http.StatusOK,
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

				if msg, ok := data["message"].(string); !ok || msg != "Note created successfully" {
					t.Errorf("Expected message 'Note created successfully', got %v", msg)
				}

				if _, hasNoteID := data["noteID"]; !hasNoteID {
					t.Error("Response missing noteID")
				}
			},
		},
		{
			name:         "Invalid Request - Empty Title",
			inputJSON:    `{"title": "", "content": "Test Content"}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid request body" {
					t.Errorf("Expected error 'Invalid request body', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid Request - Empty Content",
			inputJSON:    `{"title": "Test Title", "content": ""}`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid request body" {
					t.Errorf("Expected error 'Invalid request body', got %q", response.Error)
				}
			},
		},
		{
			name:         "Invalid JSON Format",
			inputJSON:    `{"title": "Test Title", "content": }`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response utils.Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response.Error != "Invalid request body" {
					t.Errorf("Expected error 'Invalid request body', got %q", response.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear collection before each test
			if err := testClient.Database("tonotes_test").Collection("notes").Drop(context.Background()); err != nil {
				t.Fatalf("Failed to clear notes collection: %v", err)
			}

			// Setup any test data if needed
			if tt.setupTestData != nil {
				if err := tt.setupTestData(); err != nil {
					t.Fatalf("Failed to setup test data: %v", err)
				}
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/notes", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			t.Logf("Test: %s", tt.name)
			t.Logf("Making request with body: %s", tt.inputJSON)

			// Count documents before request
			count, err := testClient.Database("tonotes_test").Collection("notes").CountDocuments(context.Background(), map[string]interface{}{})
			if err != nil {
				t.Fatalf("Failed to count documents: %v", err)
			}
			t.Logf("Documents in collection before request: %d", count)

			// Serve the request
			router.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Count documents after request
			count, err = testClient.Database("tonotes_test").Collection("notes").CountDocuments(context.Background(), map[string]interface{}{})
			if err != nil {
				t.Fatalf("Failed to count documents: %v", err)
			}
			t.Logf("Documents in collection after request: %d", count)

			// Check status code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Run custom response checks
			tt.checkResponse(t, w)
		})
	}
}
