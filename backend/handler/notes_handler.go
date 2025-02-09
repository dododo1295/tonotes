package handler

import (
	"fmt"
	"main/dto"
	"main/model"
	"main/usecase"
	"main/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type NoteHandler struct {
	service *usecase.NoteService
}

func NewNoteHandler(service *usecase.NoteService) *NoteHandler {
	return &NoteHandler{
		service: service,
	}
}

func (h *NoteHandler) getNoteLinks(baseURL string, note *model.Note) map[string]dto.NoteLink {
	links := make(map[string]dto.NoteLink)
	noteURL := baseURL + "/note/" + note.ID

	links["self"] = dto.NoteLink{Href: noteURL, Method: http.MethodGet}
	links["update"] = dto.NoteLink{Href: noteURL, Method: http.MethodPut}
	links["delete"] = dto.NoteLink{Href: noteURL, Method: http.MethodDelete}
	links["favorite"] = dto.NoteLink{Href: noteURL + "/favorite", Method: http.MethodPost}
	links["pin"] = dto.NoteLink{Href: noteURL + "/pin", Method: http.MethodPost}
	links["archive"] = dto.NoteLink{Href: noteURL + "/archive", Method: http.MethodPost}
	links["pin-position"] = dto.NoteLink{Href: noteURL + "/pin-position", Method: http.MethodPut}

	return links
}

func (h *NoteHandler) SearchNotes(c *gin.Context) {
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	query := c.Query("q")
	tags := c.QueryArray("tags")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")
	matchAll := c.Query("match_all") == "true"

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	searchOpts := usecase.NoteSearchOptions{
		UserID:    userID,
		Query:     query,
		Tags:      tags,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		MatchAll:  matchAll,
		Page:      page,
		PageSize:  pageSize,
	}

	notes, totalCount, err := h.service.SearchNotes(c.Request.Context(), searchOpts)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	pageCount := (totalCount + pageSize - 1) / pageSize

	responseLinks := map[string]dto.NoteLink{
		"self":   {Href: baseURL + "/note", Method: http.MethodGet},
		"create": {Href: baseURL + "/note", Method: http.MethodPost},
	}

	// Add pagination links
	if page < pageCount {
		nextPageURL := fmt.Sprintf("%s/note?page=%d&page_size=%d", baseURL, page+1, pageSize)

		// Add optional query parameters to the links
		if query != "" {
			nextPageURL = fmt.Sprintf("%s&q=%s", nextPageURL, query)
		}

		responseLinks["next"] = dto.NoteLink{Href: nextPageURL, Method: http.MethodGet}

	}
	if page > 1 {
		prevPageURL := fmt.Sprintf("%s/note?page=%d&page_size=%d", baseURL, page-1, pageSize)

		// Add optional query parameters to the links
		if query != "" {
			prevPageURL = fmt.Sprintf("%s&q=%s", prevPageURL, query)
		}

		responseLinks["prev"] = dto.NoteLink{Href: prevPageURL, Method: http.MethodGet}
	}
	if totalCount == 0 {
		delete(responseLinks, "next")
		delete(responseLinks, "prev")
	}

	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		pageCount,
		page,
		responseLinks,
		func(note *model.Note) map[string]dto.NoteLink {
			return h.getNoteLinks(baseURL, note)
		},
	)
	utils.Success(c, response)
}

func (h *NoteHandler) CreateNote(c *gin.Context) {
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	var note model.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	note.UserID = userID
	if err := h.service.CreateNote(c, &note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	links := h.getNoteLinks(baseURL, &note) // Pass a pointer to note

	response := dto.ToNoteResponse(&note, links)
	utils.Success(c, response)
}

func (h *NoteHandler) UpdateNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	var updates model.Note
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.UpdateNote(c, noteID, userID, &updates); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Fetch the updated note to generate links based on the updated state
	note, err := h.service.NoteRepo.GetNote(noteID, userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch updated note")
		return
	}

	links := h.getNoteLinks(baseURL, note)
	response := dto.ToNoteResponse(note, links)
	utils.Success(c, response)

}

func (h *NoteHandler) GetUserNotes(c *gin.Context) {
	userID := c.GetString("userID")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	baseURL := utils.GetBaseURL(c)

	notes, totalCount, err := h.service.GetUserNotes(c.Request.Context(), userID, limit)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	pageSize := limit
	if pageSize == 0 {
		pageSize = len(notes)
	}

	responseLinks := map[string]dto.NoteLink{
		"self":   {Href: baseURL + "/note", Method: http.MethodGet},
		"create": {Href: baseURL + "/note", Method: http.MethodPost},
	}
	pageCount := (totalCount + pageSize - 1) / pageSize
	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		pageCount,
		1,
		responseLinks,
		func(note *model.Note) map[string]dto.NoteLink {
			return h.getNoteLinks(baseURL, note)
		},
	)
	utils.Success(c, response)
}

func (h *NoteHandler) GetArchivedNotes(c *gin.Context) {
	userID := c.GetString("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	baseURL := utils.GetBaseURL(c)

	notes, totalCount, err := h.service.GetArchivedNotes(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	pageCount := (totalCount + pageSize - 1) / pageSize

	responseLinks := map[string]dto.NoteLink{
		"self": {Href: baseURL + "/note/archived", Method: http.MethodGet},
	}

	// Add pagination links
	if page < pageCount {
		nextPageURL := fmt.Sprintf("%s/note/archived?page=%d&page_size=%d", baseURL, page+1, pageSize)
		responseLinks["next"] = dto.NoteLink{Href: nextPageURL, Method: http.MethodGet}
	}

	if page > 1 {
		prevPageURL := fmt.Sprintf("%s/note/archived?page=%d&page_size=%d", baseURL, page-1, pageSize)
		responseLinks["prev"] = dto.NoteLink{Href: prevPageURL, Method: http.MethodGet}
	}
	if totalCount == 0 {
		delete(responseLinks, "next")
		delete(responseLinks, "prev")
	}
	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		pageCount,
		page,
		responseLinks,
		func(note *model.Note) map[string]dto.NoteLink {
			return h.getNoteLinks(baseURL, note)
		},
	)
	utils.Success(c, response)
}

func (h *NoteHandler) ToggleFavorite(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	if err := h.service.ToggleFavorite(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	note, err := h.service.NoteRepo.GetNote(noteID, userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch updated note")
		return
	}

	links := h.getNoteLinks(baseURL, note)
	response := dto.ToNoteResponse(note, links)
	utils.Success(c, response)

}

func (h *NoteHandler) TogglePin(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	if err := h.service.ToggleNotePin(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	note, err := h.service.NoteRepo.GetNote(noteID, userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch updated note")
		return
	}

	links := h.getNoteLinks(baseURL, note)
	response := dto.ToNoteResponse(note, links)
	utils.Success(c, response)
}

func (h *NoteHandler) ArchiveNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	if err := h.service.ArchiveNote(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	note, err := h.service.NoteRepo.GetNote(noteID, userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch updated note")
		return
	}

	links := h.getNoteLinks(baseURL, note)
	response := dto.ToNoteResponse(note, links)
	utils.Success(c, response)

}

func (h *NoteHandler) DeleteNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	err := h.service.DeleteNote(c.Request.Context(), noteID, userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Create a link to the collection after successful deletion
	responseLinks := map[string]dto.NoteLink{
		"collection": {Href: baseURL + "/note", Method: http.MethodGet},
		"create":     {Href: baseURL + "/note", Method: http.MethodPost},
	}

	utils.Success(c, gin.H{
		"message": "Note deleted successfully",
		"_links":  responseLinks, // Include links
	})
}

func (h *NoteHandler) UpdatePinPosition(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	var req struct {
		Position int `json:"position"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.UpdatePinPosition(c, noteID, userID, req.Position); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	note, err := h.service.NoteRepo.GetNote(noteID, userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch updated note")
		return
	}
	links := h.getNoteLinks(baseURL, note)
	response := dto.ToNoteResponse(note, links)
	utils.Success(c, response)

}

func (h *NoteHandler) GetUserTags(c *gin.Context) {
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	tagCounts, err := h.service.GetUserTags(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch user tags")
		return
	}

	responseLinks := map[string]dto.NoteLink{
		"self": {Href: baseURL + "/note/tag", Method: http.MethodGet},
	}

	utils.Success(c, gin.H{
		"tags":   tagCounts,
		"_links": responseLinks,
	})
}

func (h *NoteHandler) GetSearchSuggestions(c *gin.Context) {
	userID := c.GetString("userID")
	query := c.Query("q")
	baseURL := utils.GetBaseURL(c)

	suggestions, err := h.service.NoteRepo.GetSearchSuggestions(userID, query)
	if err != nil {
		utils.InternalError(c, "Failed to fetch search suggestions")
		return
	}

	responseLinks := map[string]dto.NoteLink{
		"self": {Href: baseURL + "/note/suggestion", Method: http.MethodGet},
	}

	utils.Success(c, gin.H{
		"suggestions": suggestions,
		"_links":      responseLinks,
	})
}

func (h *NoteHandler) GetPinnedNotes(c *gin.Context) {
	userID := c.GetString("userID")
	baseURL := utils.GetBaseURL(c)

	notes, err := h.service.NoteRepo.GetPinnedNotes(userID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch pinned notes")
		return
	}

	responseLinks := map[string]dto.NoteLink{
		"self": {Href: baseURL + "/note/pinned", Method: http.MethodGet},
	}

	responses := dto.ToNoteResponses(notes, func(note *model.Note) map[string]dto.NoteLink {
		return h.getNoteLinks(baseURL, note)
	})

	utils.Success(c, gin.H{
		"notes":  responses,
		"_links": responseLinks,
	})
}
