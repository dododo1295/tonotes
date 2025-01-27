package test

import (
	"bytes"
	"context"
	"encoding/json"
	"main/handler"
	"main/repository"
	"main/test/testutils"
	"main/usecase"
	"main/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateNoteHandler(t *testing.T) {
	// Use testutils for environment and database setup
	testutils.SetupTestEnvironment()
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	utils.MongoClient = client

	// Get database reference using environment variable
	db := client.Database(os.Getenv("MONGO_DB"))

	// Create notes collection explicitly
	err := db.CreateCollection(context.Background(), "notes")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Fatalf("Failed to create notes collection: %v", err)
	}

	// Initialize repository with correct database reference
	notesRepo := repository.GetNotesRepo(client)
	notesRepo.MongoCollection = db.Collection("notes") // Explicitly set collection

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
			if err := db.Collection("notes").Drop(context.Background()); err != nil {
				t.Logf("Warning dropping collection: %v", err)
			}

			// Recreate collection
			err := db.CreateCollection(context.Background(), "notes")
			if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
				t.Fatalf("Failed to create notes collection: %v", err)
			}

			// Setup test data if needed
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
			count, err := db.Collection("notes").CountDocuments(context.Background(), map[string]interface{}{})
			if err != nil {
				t.Fatalf("Failed to count documents: %v", err)
			}
			t.Logf("Documents in collection before request: %d", count)

			// Serve the request
			router.ServeHTTP(w, req)

			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Count documents after request
			count, err = db.Collection("notes").CountDocuments(context.Background(), map[string]interface{}{})
			if err != nil {
				t.Fatalf("Failed to count documents: %v", err)
			}
			t.Logf("Documents in collection after request: %d", count)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			tt.checkResponse(t, w)
		})
	}
}
