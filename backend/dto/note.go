package dto

import (
	"main/model"
	"time"
)

type NoteLink struct {
	Href   string `json:"href"`
	Method string `json:"method,omitempty"` // Optional: GET, POST, PUT, PATCH, DELETE
}

type NoteResponse struct {
	ID             string              `json:"id"`
	Title          string              `json:"title"`
	Content        string              `json:"content"`
	Tags           []string            `json:"tags,omitempty"`
	IsPinned       bool                `json:"is_pinned"`
	IsArchived     bool                `json:"is_archived"`
	PinnedPosition *int                `json:"pinned_position,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	Links          map[string]NoteLink `json:"_links,omitempty"`
}

type NotesPageResponse struct {
	Notes       []NoteResponse      `json:"notes"`
	TotalCount  int                 `json:"total_count"`
	PageCount   int                 `json:"page_count"`
	CurrentPage int                 `json:"current_page"`
	Links       map[string]NoteLink `json:"_links,omitempty"`
}

// Convert a single note to NoteResponse
func ToNoteResponse(note *model.Note, links map[string]NoteLink) NoteResponse {
	response := NoteResponse{
		ID:         note.ID,
		Title:      note.Title,
		Content:    note.Content,
		Tags:       note.Tags,
		IsPinned:   note.IsPinned,
		IsArchived: note.IsArchived,
		CreatedAt:  note.CreatedAt,
		UpdatedAt:  note.UpdatedAt,
		Links:      links, // Set links
	}

	if note.PinnedPosition != 0 {
		position := note.PinnedPosition
		response.PinnedPosition = &position
	}

	return response
}

// Convert slice of notes to slice of NoteResponse
func ToNoteResponses(notes []*model.Note, getNoteLinks func(note *model.Note) map[string]NoteLink) []NoteResponse {
	responses := make([]NoteResponse, len(notes))
	for i, note := range notes {
		responses[i] = ToNoteResponse(note, getNoteLinks(note))
	}
	return responses
}

// Convert NotesResponse to NotesPageResponse
func NewNotesPageResponse(notes []*model.Note, totalCount, pageCount, currentPage int, links map[string]NoteLink, getNoteLinks func(note *model.Note) map[string]NoteLink) *NotesPageResponse {
	return &NotesPageResponse{
		Notes:       ToNoteResponses(notes, getNoteLinks),
		TotalCount:  totalCount,
		PageCount:   pageCount,
		CurrentPage: currentPage,
		Links:       links,
	}
}
