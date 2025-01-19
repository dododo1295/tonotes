package usecase

import (
	"context"
	"errors"
	"fmt"
	"main/model"
	"main/repository"
	"sort"
	"strings"
	"time"
)

type NotesService struct {
	NotesRepo *repository.NotesRepo
}

// Searching/Filtering options for notes
type NoteSearchOptions struct {
	UserID    string
	Query     string   // For text search
	Tags      []string // For filtering by tags
	SortBy    string   // e.g., "created_at", "updated_at", "title"
	SortOrder string   // "asc" or "desc"
	MatchAll  bool     // For tag matching (AND vs OR)
	Page      int
	PageSize  int
}

type NotesResponse struct {
	Notes       []*model.Notes
	TotalCount  int
	PageCount   int
	CurrentPage int
}

// helper functions
func sortNotes(notes []*model.Notes, sortBy string, sortOrder string) {
	sort.Slice(notes, func(i, j int) bool {
		descending := sortOrder == "desc"
		switch sortBy {
		case "title":
			if descending {
				return notes[i].Title > notes[j].Title
			}
			return notes[i].Title < notes[j].Title
		case "updated_at":
			if descending {
				return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
			}
			return notes[i].UpdatedAt.Before(notes[j].UpdatedAt)
		default: // created_at
			if descending {
				return notes[i].CreatedAt.After(notes[j].CreatedAt)
			}
			return notes[i].CreatedAt.Before(notes[j].CreatedAt)
		}
	})
}

func (s *NotesService) validateNote(note *model.Notes) error {
	if note.Title == "" {
		return errors.New("note title is required")
	}
	if len(note.Title) > 200 {
		return errors.New("note title exceeds maximum length")
	}
	if len(note.Content) > 50000 {
		return errors.New("note content exceeds maximum length")
	}
	if len(note.Tags) > 10 {
		return errors.New("maximum 10 tags allowed")
	}
	return nil
}

func (s *NotesService) processTags(tags []string) []string {
	// Deduplicate tags
	seen := make(map[string]bool)
	uniqueTags := []string{}

	for _, tag := range tags {
		if !seen[tag] {
			seen[tag] = true
			// Trim and lowercase tags
			processedTag := strings.ToLower(strings.TrimSpace(tag))
			if processedTag != "" {
				uniqueTags = append(uniqueTags, processedTag)
			}
		}
	}
	return uniqueTags
}

// service functions
func (svc *NotesService) SearchNotes(ctx context.Context, opts NoteSearchOptions) (*NotesResponse, error) {
	// Basic validation
	if opts.UserID == "" {
		return nil, errors.New("user ID is required")
	}

	// Add minimum query length validation
	if opts.Query != "" && len(opts.Query) < 2 {
		return nil, errors.New("search query must be at least 2 characters")
	}

	// Validate pagination parameters
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 10 // Default page size
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100 // Maximum page size
	}

	// Process tags if provided
	if len(opts.Tags) > 0 {
		processedTags := make([]string, 0)
		for _, tag := range opts.Tags {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				processedTags = append(processedTags, strings.ToLower(trimmed))
			}
		}
		opts.Tags = processedTags
	}

	// Convert search options to repository format
	repoOpts := repository.NotesSearchOptions{
		UserID:      opts.UserID,
		Query:       strings.TrimSpace(opts.Query),
		Tags:        opts.Tags,
		MatchAll:    opts.MatchAll,
		SearchScore: opts.Query != "", // Include score when searching
		SortBy:      opts.SortBy,      // Add this
		SortOrder:   opts.SortOrder,   // Add this
	}

	// Get notes from repository
	notes, err := svc.NotesRepo.FindNotes(repoOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search notes: %w", err)
	}

	// If sort options are provided, sort the results
	if opts.SortBy != "" {
		// Don't sort if we're using text search (already sorted by relevance)
		if !repoOpts.SearchScore {
			sortNotes(notes, opts.SortBy, opts.SortOrder)
		}
	}

	// Handle pagination
	totalCount := len(notes)
	pageCount := (totalCount + opts.PageSize - 1) / opts.PageSize

	start := (opts.Page - 1) * opts.PageSize
	if start >= totalCount {
		// If requested page is beyond available results, return empty page
		return &NotesResponse{
			Notes:       []*model.Notes{},
			TotalCount:  totalCount,
			PageCount:   pageCount,
			CurrentPage: opts.Page,
		}, nil
	}

	end := start + opts.PageSize
	if end > totalCount {
		end = totalCount
	}

	// Prepare paginated results
	pagedNotes := notes[start:end]

	// Return response with metadata
	return &NotesResponse{
		Notes:       pagedNotes,
		TotalCount:  totalCount,
		PageCount:   pageCount,
		CurrentPage: opts.Page,
	}, nil
}

func (svc *NotesService) CreateNote(ctx context.Context, note *model.Notes) error {
	// Validate note content
	if err := svc.validateNote(note); err != nil {
		return err
	}

	// Check user's note limit (if any)
	count, err := svc.NotesRepo.CountUserNotes(note.UserID)
	if err != nil {
		return err
	}
	if count >= 100 {
		return errors.New("user has reached maximum note limit")
	}

	// Sanitize and process tags
	note.Tags = svc.processTags(note.Tags)

	// Set timestamps
	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now

	return svc.NotesRepo.CreateNote(note)
}

func (svc *NotesService) UpdateNote(ctx context.Context, noteID string, userID string, updates *model.Notes) error {
	// Verify note ownership first
	existing, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("note not found")
	}

	// Validate updates
	if err := svc.validateNote(updates); err != nil {
		return err
	}

	// Process tags
	updates.Tags = svc.processTags(updates.Tags)

	// Preserve certain fields from existing note
	updates.ID = existing.ID
	updates.UserID = existing.UserID
	updates.CreatedAt = existing.CreatedAt
	updates.IsPinned = existing.IsPinned
	updates.IsArchived = existing.IsArchived
	updates.PinnedPosition = existing.PinnedPosition

	// Update the note through repository
	return svc.NotesRepo.UpdateNote(noteID, userID, updates)
}

func (svc *NotesService) ToggleFavorite(ctx context.Context, noteID string, userID string) error {
	// Verify note ownership first
	note, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}
	if note == nil {
		return errors.New("note not found")
	}

	// Check if note is already favorited
	isFavorited := false
	for _, tag := range note.Tags {
		if tag == "favorites" {
			isFavorited = true
			break
		}
	}

	// Create new tags slice
	var newTags []string
	if isFavorited {
		// Remove favorites tag
		for _, tag := range note.Tags {
			if tag != "favorites" {
				newTags = append(newTags, tag)
			}
		}
	} else {
		// Add favorites tag
		newTags = append(note.Tags, "favorites")
	}

	// Create updates
	updates := &model.Notes{
		Title:      note.Title,      // Preserve existing title
		Content:    note.Content,    // Preserve existing content
		Tags:       newTags,         // Updated tags
		IsPinned:   note.IsPinned,   // Preserve pin status
		IsArchived: note.IsArchived, // Preserve archive status
	}

	return svc.NotesRepo.UpdateNote(noteID, userID, updates)
}

func (svc *NotesService) ToggleNotePin(ctx context.Context, noteID string, userID string) error {
	// Check if user has reached pinned notes limit
	pinnedNotes, err := svc.NotesRepo.GetPinnedNotes(userID)
	if err != nil {
		return err
	}

	note, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}

	// Business rule: Maximum 5 pinned notes
	if len(pinnedNotes) >= 5 && !note.IsPinned {
		return errors.New("maximum pinned notes limit reached")
	}

	return svc.NotesRepo.TogglePin(noteID, userID)
}

func (svc *NotesService) GetUserNotes(ctx context.Context, userID string, limit int) ([]*model.Notes, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	notes, err := svc.NotesRepo.GetUserNotes(userID)
	if err != nil {
		return nil, err
	}

	// Apply limit if specified
	if limit > 0 && limit < len(notes) {
		return notes[:limit], nil
	}
	return notes, nil
}

func (svc *NotesService) DeleteNote(ctx context.Context, noteID, userID string) error {
	// First verify the note exists and belongs to user
	note, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}
	if note == nil {
		return errors.New("note not found")
	}

	//Cannot delete pinned notes
	if note.IsPinned {
		return errors.New("cannot delete pinned note - unpin first")
	}

	return svc.NotesRepo.DeleteNote(noteID, userID)
}

func (svc *NotesService) ArchiveNote(ctx context.Context, noteID, userID string) error {
	// Verify note ownership first
	note, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}
	if note == nil {
		return errors.New("note not found")
	}

	// Business rule: Cannot archive pinned notes
	if note.IsPinned {
		return errors.New("cannot archive pinned note - unpin first")
	}

	return svc.NotesRepo.ArchiveNote(noteID, userID)
}

func (svc *NotesService) GetArchivedNotes(ctx context.Context, userID string, page, pageSize int) (*NotesResponse, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	notes, err := svc.NotesRepo.GetArchivedNotes(userID)
	if err != nil {
		return nil, err
	}

	// Handle pagination
	totalCount := len(notes)
	pageCount := (totalCount + pageSize - 1) / pageSize

	start := (page - 1) * pageSize
	if start >= totalCount {
		return nil, errors.New("page number exceeds available pages")
	}

	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}

	return &NotesResponse{
		Notes:       notes[start:end],
		TotalCount:  totalCount,
		PageCount:   pageCount,
		CurrentPage: page,
	}, nil
}

func (svc *NotesService) UpdatePinPosition(ctx context.Context, noteID, userID string, newPosition int) error {
	// Verify note exists and is pinned
	note, err := svc.NotesRepo.GetNote(noteID, userID)
	if err != nil {
		return err
	}
	if !note.IsPinned {
		return errors.New("note is not pinned")
	}

	// Get total pinned notes to validate position
	pinnedNotes, err := svc.NotesRepo.GetPinnedNotes(userID)
	if err != nil {
		return err
	}

	// Validate new position
	if newPosition < 1 || newPosition > len(pinnedNotes) {
		return errors.New("invalid position")
	}

	return svc.NotesRepo.UpdatePinPosition(noteID, userID, newPosition)
}

func (svc *NotesService) GetUserTags(ctx context.Context, userID string) (map[string]int, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	// Get tag counts
	tagCounts, err := svc.NotesRepo.CountNotesByTag(userID)
	if err != nil {
		return nil, err
	}

	return tagCounts, nil
}

func (svc *NotesService) GetAllUserTags(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	return svc.NotesRepo.GetAllTags(userID)
}

func (svc *NotesService) GetSearchSuggestions(ctx context.Context, userID string, prefix string) ([]string, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}
	if prefix == "" {
		return nil, errors.New("search prefix is required")
	}

	// Get suggestions from repository
	suggestions, err := svc.NotesRepo.GetSearchSuggestions(userID, prefix)
	if err != nil {
		return nil, err
	}

	// Deduplicate suggestions and ensure we only return relevant ones
	seen := make(map[string]bool)
	uniqueSuggestions := []string{}

	for _, suggestion := range suggestions {
		// Convert to lower case for comparison
		suggestion = strings.ToLower(suggestion)
		if strings.HasPrefix(suggestion, strings.ToLower(prefix)) && !seen[suggestion] {
			seen[suggestion] = true
			uniqueSuggestions = append(uniqueSuggestions, suggestion)
		}
	}

	// Sort suggestions alphabetically
	sort.Strings(uniqueSuggestions)

	return uniqueSuggestions, nil
}
