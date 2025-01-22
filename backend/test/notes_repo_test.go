package test

import (
	"context"
	"fmt"
	"main/model"
	"main/repository"
	"main/utils"
	"os"
	"strings"
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

func TestCountUserNotes(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create several test notes
	for i := 0; i < 3; i++ {
		note := createTestNote(userID)
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	count, err := notesRepo.CountUserNotes(userID)
	if err != nil {
		t.Fatalf("CountUserNotes() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountUserNotes() got = %v, want %v", count, 3)
	}
}

func TestGetAllTags(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	expectedTags := []string{"tag1", "tag2", "tag3"}

	// Create notes with different tags
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
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tags, err := notesRepo.GetAllTags(userID)
	if err != nil {
		t.Fatalf("GetAllTags() error = %v", err)
	}

	if len(tags) != len(expectedTags) {
		t.Errorf("GetAllTags() got %v tags, want %v", len(tags), len(expectedTags))
	}

	// Check if all expected tags are present
	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}
	for _, expectedTag := range expectedTags {
		if !tagMap[expectedTag] {
			t.Errorf("GetAllTags() missing expected tag: %v", expectedTag)
		}
	}
}

func TestTogglePin(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	note := createTestNote(userID)

	// Create initial note
	err := notesRepo.CreateNote(note)
	if err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	// Test pin
	err = notesRepo.TogglePin(note.ID, userID)
	if err != nil {
		t.Fatalf("TogglePin() error = %v", err)
	}

	// Verify pin
	pinnedNote, err := notesRepo.GetNote(note.ID, userID)
	if err != nil {
		t.Fatalf("Failed to get note: %v", err)
	}
	if !pinnedNote.IsPinned {
		t.Error("TogglePin() failed to pin note")
	}

	// Test unpin
	err = notesRepo.TogglePin(note.ID, userID)
	if err != nil {
		t.Fatalf("TogglePin() error = %v", err)
	}

	// Verify unpin
	unpinnedNote, err := notesRepo.GetNote(note.ID, userID)
	if err != nil {
		t.Fatalf("Failed to get note: %v", err)
	}
	if unpinnedNote.IsPinned {
		t.Error("TogglePin() failed to unpin note")
	}
}
func TestCountNotesByTag(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create notes with different tags
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
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tagCounts, err := notesRepo.CountNotesByTag(userID)
	if err != nil {
		t.Fatalf("CountNotesByTag() error = %v", err)
	}

	expectedCounts := map[string]int{
		"tag1": 1,
		"tag2": 2,
		"tag3": 1,
	}

	for tag, expectedCount := range expectedCounts {
		if count := tagCounts[tag]; count != expectedCount {
			t.Errorf("CountNotesByTag() for %s = %d, want %d",
				tag, count, expectedCount)
		}
	}
}

func TestArchiveOperationsRepo(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	note := createTestNote(userID)

	// Create note
	err := notesRepo.CreateNote(note)
	if err != nil {
		t.Fatalf("Failed to create test note: %v", err)
	}

	// Test archive
	err = notesRepo.ArchiveNote(note.ID, userID)
	if err != nil {
		t.Fatalf("ArchiveNote() error = %v", err)
	}

	// Verify archive
	archivedNotes, err := notesRepo.GetArchivedNotes(userID)
	if err != nil {
		t.Fatalf("GetArchivedNotes() error = %v", err)
	}
	if len(archivedNotes) != 1 {
		t.Errorf("Expected 1 archived note, got %d", len(archivedNotes))
	}

	// Test unarchive
	err = notesRepo.ArchiveNote(note.ID, userID)
	if err != nil {
		t.Fatalf("ArchiveNote() error = %v", err)
	}

	// Verify unarchive
	archivedNotes, err = notesRepo.GetArchivedNotes(userID)
	if err != nil {
		t.Fatalf("GetArchivedNotes() error = %v", err)
	}
	if len(archivedNotes) != 0 {
		t.Errorf("Expected 0 archived notes, got %d", len(archivedNotes))
	}
}

func TestGetUserNotes(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()
	otherUserID := uuid.New().String()

	// Create notes for main user
	for i := 0; i < 3; i++ {
		note := &model.Notes{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   fmt.Sprintf("User Note %d", i+1),
			Content: fmt.Sprintf("Content %d", i+1),
			Tags:    []string{"test"},
		}
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	// Create note for other user
	otherNote := &model.Notes{
		ID:      uuid.New().String(),
		UserID:  otherUserID,
		Title:   "Other User Note",
		Content: "Other Content",
	}
	err := notesRepo.CreateNote(otherNote)
	if err != nil {
		t.Fatalf("Failed to create other user note: %v", err)
	}

	// Test getting user notes
	notes, err := notesRepo.GetUserNotes(userID)
	if err != nil {
		t.Fatalf("GetUserNotes() error = %v", err)
	}

	if len(notes) != 3 {
		t.Errorf("GetUserNotes() got %v notes, want %v", len(notes), 3)
	}

	// Verify notes belong to correct user
	for _, note := range notes {
		if note.UserID != userID {
			t.Errorf("GetUserNotes() returned note with userID = %v, want %v",
				note.UserID, userID)
		}
	}
}

func TestUpdatePinPosition(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create and pin multiple notes
	notes := make([]*model.Notes, 3)
	for i := 0; i < 3; i++ {
		notes[i] = &model.Notes{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   fmt.Sprintf("Note %d", i+1),
			Content: "Test Content",
		}
		err := notesRepo.CreateNote(notes[i])
		if err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}
		err = notesRepo.TogglePin(notes[i].ID, userID)
		if err != nil {
			t.Fatalf("Failed to pin note: %v", err)
		}
	}

	// Test updating pin position
	tests := []struct {
		name        string
		noteID      string
		newPosition int
		wantErr     bool
	}{
		{
			name:        "Valid Position Update",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := notesRepo.UpdatePinPosition(tt.noteID, userID, tt.newPosition)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePinPosition() error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestGetPinnedNotes(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create mix of pinned and unpinned notes
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Pinned Note 1",
			Content: "Content 1",
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Unpinned Note",
			Content: "Content 2",
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Pinned Note 2",
			Content: "Content 3",
		},
	}

	for _, note := range notes {
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}
	}

	// Pin specific notes
	err := notesRepo.TogglePin(notes[0].ID, userID)
	if err != nil {
		t.Fatalf("Failed to pin note: %v", err)
	}
	err = notesRepo.TogglePin(notes[2].ID, userID)
	if err != nil {
		t.Fatalf("Failed to pin note: %v", err)
	}

	// Get pinned notes
	pinnedNotes, err := notesRepo.GetPinnedNotes(userID)
	if err != nil {
		t.Fatalf("GetPinnedNotes() error = %v", err)
	}

	if len(pinnedNotes) != 2 {
		t.Errorf("GetPinnedNotes() got %v notes, want %v", len(pinnedNotes), 2)
	}

	// Verify all returned notes are pinned
	for _, note := range pinnedNotes {
		if !note.IsPinned {
			t.Error("GetPinnedNotes() returned unpinned note")
		}
	}
}

func TestGetSearchSuggestions(t *testing.T) {
	_, notesRepo, cleanup := setupNotesTest(t)
	defer cleanup()

	userID := uuid.New().String()

	// Create notes with specific titles and tags
	notes := []*model.Notes{
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Programming Basics",
			Content: "Content 1",
			Tags:    []string{"programming", "basics"},
		},
		{
			ID:      uuid.New().String(),
			UserID:  userID,
			Title:   "Project Management",
			Content: "Content 2",
			Tags:    []string{"project", "management"},
		},
	}

	for _, note := range notes {
		err := notesRepo.CreateNote(note)
		if err != nil {
			t.Fatalf("Failed to create test note: %v", err)
		}
	}

	tests := []struct {
		name          string
		prefix        string
		expectedTerms []string
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "Search with 'pro' prefix",
			prefix:        "pro",
			expectedTerms: []string{"programming", "project"},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "Search with 'man' prefix",
			prefix:        "man",
			expectedTerms: []string{"management"},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name:          "Empty prefix",
			prefix:        "",
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, err := notesRepo.GetSearchSuggestions(userID, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSearchSuggestions() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}

			if len(suggestions) != tt.expectedCount {
				t.Errorf("GetSearchSuggestions() got %v suggestions, want %v",
					len(suggestions), tt.expectedCount)
			}

			// Verify expected terms are present
			for _, term := range tt.expectedTerms {
				found := false
				for _, suggestion := range suggestions {
					if strings.EqualFold(suggestion, term) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetSearchSuggestions() missing expected term: %v",
						term)
				}
			}
		})
	}
}
