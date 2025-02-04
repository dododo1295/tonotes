package dto

import (
	"main/model"
	"time"
)

type NoteResponse struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Tags           []string  `json:"tags,omitempty"`
	IsPinned       bool      `json:"is_pinned"`
	IsArchived     bool      `json:"is_archived"`
	PinnedPosition *int      `json:"pinned_position,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type NotesPageResponse struct {
	Notes       []NoteResponse `json:"notes"`
	TotalCount  int            `json:"total_count"`
	PageCount   int            `json:"page_count"`
	CurrentPage int            `json:"current_page"`
}

// Convert a single note to NoteResponse
func ToNoteResponse(note *model.Notes) NoteResponse {
	response := NoteResponse{
		ID:         note.ID,
		Title:      note.Title,
		Content:    note.Content,
		Tags:       note.Tags,
		IsPinned:   note.IsPinned,
		IsArchived: note.IsArchived,
		CreatedAt:  note.CreatedAt,
		UpdatedAt:  note.UpdatedAt,
	}

	if note.PinnedPosition != 0 {
		position := note.PinnedPosition
		response.PinnedPosition = &position
	}

	return response
}

// Convert slice of notes to slice of NoteResponse
func ToNoteResponses(notes []*model.Notes) []NoteResponse {
	responses := make([]NoteResponse, len(notes))
	for i, note := range notes {
		responses[i] = ToNoteResponse(note)
	}
	return responses
}

// Convert NotesResponse to NotesPageResponse
func NewNotesPageResponse(notes []*model.Notes, totalCount, pageCount, currentPage int) *NotesPageResponse {
	return &NotesPageResponse{
		Notes:       ToNoteResponses(notes),
		TotalCount:  totalCount,
		PageCount:   pageCount,
		CurrentPage: currentPage,
	}
}
