package test

import (
	"context"
	"main/model"
	"main/repository"
	"main/utils"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	os.Setenv("GO_ENV", "test")
	os.Setenv("MONGO_DB", "tonotes_test")
}

func setupNotesTest(t *testing.T) (*mongo.Client, *repository.NotesRepo, func()) {
	// Connect to MongoDB
	client, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	utils.MongoClient = client

	// Create indexes before creating the repository
	db := client.Database("tonotes_test")
	err = repository.SetupIndexes(db)
	if err != nil {
		t.Fatalf("Failed to setup indexes: %v", err)
	}

	notesRepo := repository.GetNotesRepo(client)

	// Return cleanup function
	cleanup := func() {
		if err := client.Database("tonotes_test").Collection("notes").Drop(context.Background()); err != nil {
			t.Errorf("Failed to clean up test collection: %v", err)
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("Failed to disconnect from MongoDB: %v", err)
		}
	}

	return client, notesRepo, cleanup
}

func createTestNote(userID string) *model.Notes {
	return &model.Notes{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     "Test Note",
		Content:   "Test Content",
		Tags:      []string{"test", "example"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestFindNotes(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create test notes
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "First Note",
			Content: "Test content 1",
			Tags:    []string{"tag1", "tag2"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Second Note",
			Content: "Test content 2",
			Tags:    []string{"tag2", "tag3"},
		},
	}

	// Insert test notes
	for _, note := range notes {
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		searchOpts    repository.NotesSearchOptions
		expectedCount int
		wantErr       bool
	}{
		{
			name: "Find All User Notes",
			searchOpts: repository.NotesSearchOptions{
				UserID: userID,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Find By Tag",
			searchOpts: repository.NotesSearchOptions{
				UserID: userID,
				Tags:   []string{"tag1"},
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Find By Query",
			searchOpts: repository.NotesSearchOptions{
				UserID:      userID,
				Query:       "First",
				SearchScore: true, // Add this to enable text search scoring
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Find With Tag Match All",
			searchOpts: repository.NotesSearchOptions{
				UserID:   userID,
				Tags:     []string{"tag2"},
				MatchAll: true,
			},
			expectedCount: 2, // This should probably be 2 since both notes have tag2
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := notesRepo.FindNotes(tt.searchOpts)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindNotes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(found) != tt.expectedCount {
				t.Errorf("FindNotes() got %v notes, want %v", len(found), tt.expectedCount)
			}
		})
	}
}

func TestCreateAndGetNote(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	note := createTestNote(userID)

	// Test CreateNote
	err := notesRepo.CreateNote(note)
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}

	// Test GetNote
	retrieved, err := notesRepo.GetNote(note.ID, userID)
	if err != nil {
		t.Fatalf("GetNote() error = %v", err)
	}

	if retrieved.Title != note.Title {
		t.Errorf("GetNote() got title = %v, want %v", retrieved.Title, note.Title)
	}
}

func TestUpdateNoteRepo(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	note := createTestNote(userID)

	// Create initial note
	err := notesRepo.CreateNote(note)
	if err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	// Update note
	updates := &model.Notes{
		Title:   "Updated Title",
		Content: "Updated Content",
		Tags:    []string{"updated"},
	}

	err = notesRepo.UpdateNote(note.ID, userID, updates)
	if err != nil {
		t.Fatalf("UpdateNote() error = %v", err)
	}

	// Verify update
	updated, err := notesRepo.GetNote(note.ID, userID)
	if err != nil {
		t.Fatalf("Failed to get updated note: %v", err)
	}

	if updated.Title != updates.Title {
		t.Errorf("UpdateNote() got title = %v, want %v", updated.Title, updates.Title)
	}
}

func TestDeleteNote(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	note := createTestNote(userID)

	// Create note
	err := notesRepo.CreateNote(note)
	if err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	// Delete note
	err = notesRepo.DeleteNote(note.ID, userID)
	if err != nil {
		t.Fatalf("DeleteNote() error = %v", err)
	}

	// Verify deletion
	_, err = notesRepo.GetNote(note.ID, userID)
	if err == nil {
		t.Error("DeleteNote() note still exists after deletion")
	}
}
