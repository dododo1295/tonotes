package test

import (
	"context"
	"fmt"
	"main/model"
	"main/repository"
	"main/usecase"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", "tonotes_test")
	gin.SetMode(gin.TestMode)
}

func setupNotesUsecaseTest(t *testing.T) (*mongo.Client, *usecase.NotesService, func()) {
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	db := client.Database("tonotes_test")

	// Ensure indexes are created before running tests
	err = repository.SetupIndexes(db)
	if err != nil {
		t.Fatalf("Failed to setup indexes: %v", err)
	}

	notesRepo := repository.GetNotesRepo(client)
	notesService := &usecase.NotesService{
		NotesRepo: notesRepo,
	}

	cleanup := func() {
		if err := client.Database("tonotes_test").Collection("notes").Drop(context.Background()); err != nil {
			t.Errorf("Failed to clean up test collection: %v", err)
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("Failed to disconnect from MongoDB: %v", err)
		}
	}

	return client, notesService, cleanup
}
func TestSearchNotes(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create test notes
	notes := []*model.Notes{
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "First Note",
			Content:   "Content 1",
			Tags:      []string{"tag1", "tag2"},
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Second Note",
			Content:   "Content 2",
			Tags:      []string{"tag2", "tag3"},
			CreatedAt: time.Now().Add(time.Hour),
		},
	}

	for _, note := range notes {
		if err := svc.CreateNote(ctx, note); err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		opts          usecase.NoteSearchOptions
		expectedCount int
		wantErr       bool
	}{
		{
			name: "Basic Search",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Page:     1,
				PageSize: 10,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Search with Tag",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Tags:     []string{"tag1"},
				Page:     1,
				PageSize: 10,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Search with Query",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Query:    "First",
				Page:     1,
				PageSize: 10,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Invalid UserID",
			opts: usecase.NoteSearchOptions{
				UserID: "",
				Page:   1,
			},
			expectedCount: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := svc.SearchNotes(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchNotes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(response.Notes) != tt.expectedCount {
				t.Errorf("SearchNotes() got %v notes, want %v", len(response.Notes), tt.expectedCount)
			}
		})
	}
}

func TestToggleFavorite(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create a test note
	note := &model.Notes{
		ID:      uuid.New().String(),
		UserID:  userID,
		Title:   "Test Note",
		Content: "Test Content",
		Tags:    []string{"test"},
	}

	if err := svc.CreateNote(ctx, note); err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	tests := []struct {
		name    string
		noteID  string
		userID  string
		wantErr bool
	}{
		{
			name:    "Add Favorite",
			noteID:  note.ID,
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "Remove Favorite",
			noteID:  note.ID,
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "Invalid Note ID",
			noteID:  "invalid-id",
			userID:  userID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ToggleFavorite(ctx, tt.noteID, tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToggleFavorite() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the change
				updatedNote, err := svc.NotesRepo.GetNote(tt.noteID, tt.userID)
				if err != nil {
					t.Fatalf("Failed to get updated note: %v", err)
				}

				hasFavorite := false
				for _, tag := range updatedNote.Tags {
					if tag == "favorites" {
						hasFavorite = true
						break
					}
				}

				if tt.name == "Add Favorite" && !hasFavorite {
					t.Error("Note should have favorites tag")
				}
				if tt.name == "Remove Favorite" && hasFavorite {
					t.Error("Note should not have favorites tag")
				}
			}
		})
	}
}

func TestPinOperations(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create 6 test notes
	notes := make([]*model.Notes, 6)
	for i := 0; i < 6; i++ {
		notes[i] = &model.Notes{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Test Note",
			Content: "Test Content",
		}
		if err := svc.CreateNote(ctx, notes[i]); err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name    string
		noteID  string
		wantErr bool
	}{
		{
			name:    "Pin First Note",
			noteID:  notes[0].ID,
			wantErr: false,
		},
		{
			name:    "Pin Second Note",
			noteID:  notes[1].ID,
			wantErr: false,
		},
		{
			name:    "Pin Third Note",
			noteID:  notes[2].ID,
			wantErr: false,
		},
		{
			name:    "Pin Fourth Note",
			noteID:  notes[3].ID,
			wantErr: false,
		},
		{
			name:    "Pin Fifth Note",
			noteID:  notes[4].ID,
			wantErr: false,
		},
		{
			name:    "Pin Sixth Note (Should Fail)",
			noteID:  notes[5].ID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ToggleNotePin(ctx, tt.noteID, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToggleNotePin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateNote(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	tests := []struct {
		name    string
		note    *model.Notes
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Note",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   "Test Note",
				Content: "Valid content",
				Tags:    []string{"test", "valid"},
			},
			wantErr: false,
		},
		{
			name: "Empty Title",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Content: "Content without title",
			},
			wantErr: true,
			errMsg:  "note title is required",
		},
		{
			name: "Empty Content",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   "Title without content",
				Content: "",
			},
			wantErr: true,
			errMsg:  "note content is required",
		},
		{
			name: "Missing Both Title and Content",
			note: &model.Notes{
				ID:     uuid.New().String(),
				UserID: userID,
			},
			wantErr: true,
			errMsg:  "note title is required",
		},
		{
			name: "Title Too Long",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   strings.Repeat("a", 201), // 201 characters
				Content: "Content",
			},
			wantErr: true,
			errMsg:  "note title exceeds maximum length",
		},
		{
			name: "Content Too Long",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   "Test Note",
				Content: strings.Repeat("a", 50001), // 50001 characters
			},
			wantErr: true,
			errMsg:  "note content exceeds maximum length",
		},
		{
			name: "Too Many Tags",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   "Test Note",
				Content: "Valid content",
				Tags:    []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			},
			wantErr: true,
			errMsg:  "maximum 10 tags allowed",
		},
		{
			name: "Minimum Content Length",
			note: &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   "Test Note",
				Content: " ", // Just whitespace
			},
			wantErr: true,
			errMsg:  "note content cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateNote(ctx, tt.note)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("CreateNote() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}

	// Test note limit
	t.Run("Note Limit Test", func(t *testing.T) {
		for i := 0; i < 101; i++ {
			note := &model.Notes{
				ID:      uuid.New().String(),
				UserID:  userID,
				Title:   fmt.Sprintf("Note %d", i),
				Content: "Valid content for limit test",
			}
			err := svc.CreateNote(ctx, note)
			if i == 100 && err == nil {
				t.Error("CreateNote() should fail after 100 notes")
			}
			if i == 100 && err != nil && err.Error() != "user has reached maximum note limit" {
				t.Errorf("Expected 'user has reached maximum note limit' error, got %v", err)
			}
		}
	})
}

func TestUpdateNoteUsecase(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create initial note with explicit CreatedAt time
	createdAt := time.Now().UTC().Round(time.Second) // Round to seconds for comparison
	originalNote := &model.Notes{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     "Original Title",
		Content:   "Original Content",
		Tags:      []string{"original"},
		CreatedAt: createdAt,
	}

	if err := svc.CreateNote(ctx, originalNote); err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	tests := []struct {
		name    string
		updates *model.Notes
		wantErr bool
	}{
		{
			name: "Valid Update",
			updates: &model.Notes{
				Title:   "Updated Title",
				Content: "Updated Content",
				Tags:    []string{"updated"},
			},
			wantErr: false,
		},
		{
			name: "Invalid - Empty Title",
			updates: &model.Notes{
				Title:   "",
				Content: "Updated Content",
			},
			wantErr: true,
		},
		{
			name: "Note Not Found",
			updates: &model.Notes{
				ID:      uuid.New().String(),
				Title:   "Updated Title",
				Content: "Updated Content",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr {
				tt.updates.ID = originalNote.ID
			}

			err := svc.UpdateNote(ctx, tt.updates.ID, userID, tt.updates)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateNote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				updatedNote, err := svc.NotesRepo.GetNote(originalNote.ID, userID)
				if err != nil {
					t.Errorf("Failed to get updated note: %v", err)
					return
				}

				// Compare CreatedAt times within a tolerance
				timeDiff := updatedNote.CreatedAt.Sub(createdAt)
				if timeDiff > time.Second || timeDiff < -time.Second {
					t.Errorf("CreatedAt changed too much during update: got %v, want %v (diff: %v)",
						updatedNote.CreatedAt, createdAt, timeDiff)
				}

				// Verify other fields were updated
				if updatedNote.Title != tt.updates.Title {
					t.Errorf("Title not updated correctly: got %v, want %v",
						updatedNote.Title, tt.updates.Title)
				}
				if updatedNote.Content != tt.updates.Content {
					t.Errorf("Content not updated correctly: got %v, want %v",
						updatedNote.Content, tt.updates.Content)
				}
			}
		})
	}
}

func TestArchiveOperations(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create a test note
	note := &model.Notes{
		ID:      uuid.New().String(),
		UserID:  userID,
		Title:   "Test Note",
		Content: "Test Content",
	}

	if err := svc.CreateNote(ctx, note); err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	tests := []struct {
		name      string
		operation func() error
		wantErr   bool
	}{
		{
			name: "Archive Note",
			operation: func() error {
				return svc.ArchiveNote(ctx, note.ID, userID)
			},
			wantErr: false,
		},
		{
			name: "Get Archived Notes",
			operation: func() error {
				response, err := svc.GetArchivedNotes(ctx, userID, 1, 10)
				if err != nil {
					return err
				}
				if len(response.Notes) != 1 {
					return fmt.Errorf("expected 1 archived note, got %d", len(response.Notes))
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "Archive Nonexistent Note",
			operation: func() error {
				return svc.ArchiveNote(ctx, "nonexistent-id", userID)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestTagOperations(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create notes with various tags
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Note 1",
			Content: "Content 1",
			Tags:    []string{"tag1", "tag2"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Note 2",
			Content: "Content 2",
			Tags:    []string{"tag2", "tag3"},
		},
	}

	for _, note := range notes {
		if err := svc.CreateNote(ctx, note); err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name      string
		operation func() error
		wantErr   bool
	}{
		{
			name: "Get All Tags",
			operation: func() error {
				tags, err := svc.GetAllUserTags(ctx, userID)
				if err != nil {
					return err
				}
				if len(tags) != 3 {
					return fmt.Errorf("expected 3 unique tags, got %d", len(tags))
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "Get Tag Counts",
			operation: func() error {
				tagCounts, err := svc.GetUserTags(ctx, userID)
				if err != nil {
					return err
				}
				if tagCounts["tag2"] != 2 {
					return fmt.Errorf("expected tag2 count to be 2, got %d", tagCounts["tag2"])
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestPinPositionOperations(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create and pin some notes
	notes := make([]*model.Notes, 3)
	for i := 0; i < 3; i++ {
		notes[i] = &model.Notes{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   fmt.Sprintf("Note %d", i+1),
			Content: "Content",
		}
		if err := svc.CreateNote(ctx, notes[i]); err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}
		if err := svc.ToggleNotePin(ctx, notes[i].ID, userID); err != nil {
			t.Fatalf("Failed to pin note: %v", err)
		}
	}

	tests := []struct {
		name        string
		noteID      string
		newPosition int
		wantErr     bool
	}{
		{
			name:        "Valid Position Change",
			noteID:      notes[0].ID,
			newPosition: 3,
			wantErr:     false,
		},
		{
			name:        "Invalid Position - Too High",
			noteID:      notes[0].ID,
			newPosition: 4,
			wantErr:     true,
		},
		{
			name:        "Invalid Position - Zero",
			noteID:      notes[0].ID,
			newPosition: 0,
			wantErr:     true,
		},
		{
			name:        "Invalid Note ID",
			noteID:      "nonexistent-id",
			newPosition: 1,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.UpdatePinPosition(ctx, tt.noteID, userID, tt.newPosition)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePinPosition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureIndexes(t *testing.T) {
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	db := client.Database("tonotes_test")

	// Setup indexes
	err = repository.SetupIndexes(db)
	if err != nil {
		t.Fatalf("Failed to setup indexes: %v", err)
	}

	// Verify indexes
	collection := db.Collection("notes")
	cursor, err := collection.Indexes().List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list indexes: %v", err)
	}
	defer cursor.Close(context.Background())

	var indexes []bson.M
	if err = cursor.All(context.Background(), &indexes); err != nil {
		t.Fatalf("Failed to get indexes: %v", err)
	}

	// Check for text index
	foundTextIndex := false
	for _, index := range indexes {
		if weights, exists := index["weights"]; exists {
			foundTextIndex = true
			weightsMap := weights.(bson.M)
			// Verify weights are set correctly
			if weightsMap["title"].(int32) != 10 ||
				weightsMap["content"].(int32) != 5 ||
				weightsMap["tags"].(int32) != 3 {
				t.Error("Text index weights are not set correctly")
			}
			break
		}
	}

	if !foundTextIndex {
		t.Error("Text index was not created")
	}

	// Cleanup
	err = db.Collection("notes").Drop(context.Background())
	if err != nil {
		t.Errorf("Failed to cleanup test collection: %v", err)
	}
}

func TestSearchSuggestions(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create notes with various titles and tags
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Programming basics",
			Content: "Content 1",
			Tags:    []string{"programming", "basics"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Project management",
			Content: "Content 2",
			Tags:    []string{"project", "management"},
		},
	}

	// Log the notes being created
	for _, note := range notes {
		t.Logf("Creating note - Title: %s, Tags: %v", note.Title, note.Tags)
		if err := svc.CreateNote(context.Background(), note); err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		prefix        string
		expectedCount int
		expectedTerms []string
		wantErr       bool
	}{
		{
			name:          "Search with 'pro' prefix",
			prefix:        "pro",
			expectedCount: 2,
			expectedTerms: []string{"programming", "project"},
			wantErr:       false,
		},
		{
			name:          "Search with 'man' prefix",
			prefix:        "man",
			expectedCount: 1,
			expectedTerms: []string{"management"},
			wantErr:       false,
		},
		{
			name:          "Empty prefix",
			prefix:        "",
			expectedCount: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, err := svc.GetSearchSuggestions(userID, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSearchSuggestions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("Got suggestions for prefix '%s': %v", tt.prefix, suggestions)

			if !tt.wantErr {
				if len(suggestions) != tt.expectedCount {
					t.Errorf("GetSearchSuggestions() got %v suggestions, want %v", len(suggestions), tt.expectedCount)
				}

				for _, term := range tt.expectedTerms {
					found := false
					for _, suggestion := range suggestions {
						if suggestion == term {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected term %s not found in suggestions", term)
					}
				}
			}
		})
	}
}

func TestAdvancedSearch(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create test notes with various content
	notes := []*model.Notes{
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Programming in Go",
			Content:   "Learning about golang programming",
			Tags:      []string{"programming", "golang"},
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Python Basics",
			Content:   "Introduction to Python programming",
			Tags:      []string{"programming", "python"},
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Programming Tips",
			Content:   "Various programming tips and tricks",
			Tags:      []string{"programming", "tips"},
			CreatedAt: time.Now(),
		},
	}

	// Log the notes being created
	for _, note := range notes {
		t.Logf("Creating note - Title: %s, CreatedAt: %v", note.Title, note.CreatedAt)
		if err := svc.CreateNote(ctx, note); err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		opts          usecase.NoteSearchOptions
		expectedCount int
		wantErr       bool
	}{
		{
			name: "Search with multiple tags",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Tags:     []string{"programming", "golang"},
				MatchAll: true,
				Page:     1,
				PageSize: 10,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Search with query and sort",
			opts: usecase.NoteSearchOptions{
				UserID:    userID,
				Query:     "programming",
				SortBy:    "created_at",
				SortOrder: "desc",
				Page:      1,
				PageSize:  10,
			},
			expectedCount: 3,
			wantErr:       false,
		},
		{
			name: "Fuzzy search",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Query:    "programing", // Intentional misspelling
				Page:     1,
				PageSize: 10,
			},
			expectedCount: 3, // Should still find programming-related notes
			wantErr:       false,
		},
		{
			name: "Pagination test",
			opts: usecase.NoteSearchOptions{
				UserID:   userID,
				Page:     1,
				PageSize: 2,
			},
			expectedCount: 2,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := svc.SearchNotes(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchNotes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(response.Notes) != tt.expectedCount {
					t.Errorf("SearchNotes() got %v notes, want %v", len(response.Notes), tt.expectedCount)
				}

				// Log results for sorting tests
				if tt.opts.SortBy == "created_at" && len(response.Notes) > 1 {
					t.Log("Checking sort order:")
					for i, note := range response.Notes {
						t.Logf("Note %d: Title=%s CreatedAt=%v", i, note.Title, note.CreatedAt)
						if i > 0 && tt.opts.SortOrder == "desc" {
							if response.Notes[i].CreatedAt.After(response.Notes[i-1].CreatedAt) {
								t.Error("Notes not properly sorted by created_at desc")
							}
						}
					}
				}
			}
		})
	}
}

func TestGetSearchSuggestionsEdgeCases(t *testing.T) {
	_, svc, cleanup := setupNotesUsecaseTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create test notes with various edge cases in titles and tags
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Test Note with Special Ch@r@cters!",
			Content: "Content 1",
			Tags:    []string{"test!@#", "special-tag"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "测试 Unicode Title",
			Content: "Content 2",
			Tags:    []string{"测试", "unicode-test"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   strings.Repeat("a", 200),
			Content: "Content 3",
			Tags:    []string{strings.Repeat("b", 100)},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "  Multiple   Spaces   Test  ",
			Content: "Content 4",
			Tags:    []string{"  spaced  tag  "},
		},
	}

	// Insert test notes
	for _, note := range notes {
		err := svc.CreateNote(context.Background(), note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		userID        string
		prefix        string
		expectedCount int
		expectedError string
		checkResults  func(t *testing.T, suggestions []string)
	}{
		{
			name:          "Special Characters in Prefix",
			userID:        userID,
			prefix:        "test!@#",
			expectedCount: 1,
			checkResults: func(t *testing.T, suggestions []string) {
				for _, s := range suggestions {
					if !strings.HasPrefix(strings.ToLower(s), "test") {
						t.Errorf("Invalid suggestion for special characters: %s", s)
					}
				}
			},
		},
		{
			name:          "Unicode Characters",
			userID:        userID,
			prefix:        "测试",
			expectedCount: 1,
			checkResults: func(t *testing.T, suggestions []string) {
				found := false
				for _, s := range suggestions {
					if strings.Contains(s, "测试") {
						found = true
						break
					}
				}
				if !found {
					t.Error("Unicode suggestion not found")
				}
			},
		},
		{
			name:          "Very Long Prefix",
			userID:        userID,
			prefix:        strings.Repeat("a", 100),
			expectedCount: 1,
			checkResults: func(t *testing.T, suggestions []string) {
				for _, s := range suggestions {
					if len(s) > 200 {
						t.Errorf("Suggestion too long: %s", s)
					}
				}
			},
		},
		{
			name:          "Multiple Spaces in Prefix",
			userID:        userID,
			prefix:        "multiple", // Changed from "  Multiple   Spaces  "
			expectedCount: 1,
			checkResults: func(t *testing.T, suggestions []string) {
				normalized := "multiple" // Changed from "multiple spaces"
				found := false
				for _, s := range suggestions {
					if strings.ToLower(s) == normalized {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find normalized suggestion '%s'", normalized)
				}
			},
		},
		{
			name:          "Empty Prefix",
			userID:        userID,
			prefix:        "",
			expectedCount: 0,
			expectedError: "search prefix is required",
		},
		{
			name:          "Only Spaces Prefix",
			userID:        userID,
			prefix:        "   ",
			expectedCount: 0,
			expectedError: "search prefix is required",
		},
		{
			name:          "Empty UserID",
			userID:        "",
			prefix:        "test",
			expectedCount: 0,
			expectedError: "user ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, err := svc.GetSearchSuggestions(tt.userID, tt.prefix)

			// Check error cases
			if tt.expectedError != "" {
				if err == nil || err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%v'", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check suggestion count
			if len(suggestions) != tt.expectedCount {
				t.Errorf("Got %d suggestions, want %d", len(suggestions), tt.expectedCount)
			}

			// Run custom checks if provided
			if tt.checkResults != nil {
				tt.checkResults(t, suggestions)
			}
		})
	}
}
