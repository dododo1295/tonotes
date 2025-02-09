package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	testutils.SetupTestEnvironment()
}

func TestCreateNoteHandler(t *testing.T) {
	// Verify environment setup
	testutils.VerifyTestEnvironment(t)

	// Setup test database
	client, cleanup := testutils.SetupTestDB(t)
	defer cleanup()

	utils.MongoClient = client

	// Get database reference using test database name
	db := client.Database(os.Getenv("MONGO_DB_TEST"))

	// Create notes collection
	ctx := context.Background()
	err := db.CreateCollection(ctx, "notes")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		t.Fatalf("Failed to create notes collection: %v", err)
	}

	// Initialize repository and service
	notesRepo := repository.GetNoteRepo(client)
	notesRepo.MongoCollection = db.Collection("notes")

	notesService := &usecase.NoteService{
		NoteRepo: notesRepo,
	}

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Add logging middleware
		t.Logf("Processing request: %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	})

	router.POST("/notes", func(c *gin.Context) {
		c.Set("userID", "test-user")
		notesHandler := handler.NewNoteHandler(notesService)
		notesHandler.CreateNote(c)
	})

	tests := []struct {
		name          string
		inputJSON     string
		expectedCode  int
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
		checkDatabase func(*testing.T, *mongo.Database)
		setupTestData func(*testing.T) error
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

				// Check required fields exist
				requiredFields := []string{"id", "title", "content", "created_at", "updated_at"}
				for _, field := range requiredFields {
					if _, exists := data[field]; !exists {
						t.Errorf("Response missing required field: %s", field)
					}
				}

				// Verify content matches input
				if title, ok := data["title"].(string); !ok || title != "Test Note" {
					t.Errorf("Expected title 'Test Note', got %v", data["title"])
				}
				if content, ok := data["content"].(string); !ok || content != "Test Content" {
					t.Errorf("Expected content 'Test Content', got %v", data["content"])
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
			checkDatabase: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("notes").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 0 {
					t.Errorf("Expected 0 documents in collection, got %d", count)
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
			checkDatabase: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("notes").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 0 {
					t.Errorf("Expected 0 documents in collection, got %d", count)
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
			checkDatabase: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("notes").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 0 {
					t.Errorf("Expected 0 documents in collection, got %d", count)
				}
			},
		},
		{
			name: "Note with Maximum Length Title",
			inputJSON: fmt.Sprintf(`{
        "title": "%s",
        "content": "Valid content"
    }`, strings.Repeat("a", 200)),
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

				if title, ok := data["title"].(string); !ok || len(title) != 200 {
					t.Errorf("Expected title length 200, got %d", len(title))
				}
			},
		},
		{
			name: "Note with Maximum Tags",
			inputJSON: `{
        "title": "Test Title",
        "content": "Test Content",
        "tags": ["tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10"]
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

				tags, ok := data["tags"].([]interface{})
				if !ok {
					t.Fatal("Response missing tags array")
				}

				if len(tags) != 10 {
					t.Errorf("Expected 10 tags, got %d", len(tags))
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
				if err := tt.setupTestData(t); err != nil {
					t.Fatalf("Failed to setup test data: %v", err)
				}
			}

			// Create request
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/notes", bytes.NewBufferString(tt.inputJSON))
			req.Header.Set("Content-Type", "application/json")

			// Log test information
			t.Logf("Running test: %s", tt.name)
			t.Logf("Request body: %s", tt.inputJSON)

			// Execute request
			router.ServeHTTP(w, req)

			// Log response
			t.Logf("Response Status: %d", w.Code)
			t.Logf("Response Body: %s", w.Body.String())

			// Check response code
			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			// Check response content
			tt.checkResponse(t, w)

			// Check database state
			if tt.checkDatabase != nil {
				tt.checkDatabase(t, db)
			}
		})
	}
}
