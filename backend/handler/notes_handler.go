package handler

import (
	"main/dto"
	"main/model"
	"main/usecase"
	"main/utils"
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
func (h *NoteHandler) SearchNotes(c *gin.Context) {
	userID := c.GetString("userID")

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

	notes, totalCount, err := h.service.SearchNotes(c, searchOpts)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		(totalCount+pageSize-1)/pageSize,
		page,
	)
	utils.Success(c, response)
}

func (h *NoteHandler) CreateNote(c *gin.Context) {
	var note model.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	note.UserID = c.GetString("userID")
	if err := h.service.CreateNote(c, &note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	response := dto.ToNoteResponse(&note)
	utils.Success(c, response)
}

func (h *NoteHandler) UpdateNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	var updates model.Note
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.UpdateNote(c, noteID, userID, &updates); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note updated successfully"})
}

func (h *NoteHandler) DeleteNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := h.service.DeleteNote(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note deleted successfully"})
}

func (h *NoteHandler) ToggleFavorite(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := h.service.ToggleFavorite(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note favorite status toggled successfully"})
}

func (h *NoteHandler) TogglePin(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := h.service.ToggleNotePin(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note pin status toggled successfully"})
}

func (h *NoteHandler) ArchiveNote(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := h.service.ArchiveNote(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note archived successfully"})
}

func (h *NoteHandler) GetUserTags(c *gin.Context) {
	userID := c.GetString("userID")

	tags, err := h.service.GetUserTags(c, userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, tags)
}

func (h *NoteHandler) GetSearchSuggestions(c *gin.Context) {
	userID := c.GetString("userID")
	prefix := c.Query("prefix")

	suggestions, err := h.service.GetSearchSuggestions(userID, prefix)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, suggestions)
}

func (h *NoteHandler) GetUserNotes(c *gin.Context) {
	userID := c.GetString("userID")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))

	notes, totalCount, err := h.service.GetUserNotes(c, userID, limit)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	pageSize := limit
	if pageSize == 0 {
		pageSize = len(notes)
	}

	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		(totalCount+pageSize-1)/pageSize,
		1,
	)
	utils.Success(c, response)
}

func (h *NoteHandler) GetArchivedNotes(c *gin.Context) {
	userID := c.GetString("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	notes, totalCount, err := h.service.GetArchivedNotes(c, userID, page, pageSize)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	response := dto.NewNotesPageResponse(
		notes,
		totalCount,
		(totalCount+pageSize-1)/pageSize,
		page,
	)
	utils.Success(c, response)
}

func (h *NoteHandler) UpdatePinPosition(c *gin.Context) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

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

	utils.Success(c, gin.H{"message": "Pin position updated successfully"})
}

func (h *NoteHandler) GetAllUserTags(c *gin.Context) {
	userID := c.GetString("userID")

	tags, err := h.service.GetAllUserTags(c, userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, tags)
}

func (h *NoteHandler) GetPinnedNotes(c *gin.Context) {
	userID := c.GetString("userID")

	notes, err := h.service.GetPinnedNotes(c.Request.Context(), userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	response := dto.ToNoteResponses(notes)
	utils.Success(c, response)
}
