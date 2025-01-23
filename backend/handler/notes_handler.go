package handler

import (
	"main/model"
	"main/usecase"
	"main/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func SearchNotesHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID") // Assuming middleware sets this

	// Parse query parameters
	query := c.Query("q")
	tags := c.QueryArray("tags")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")
	matchAll := c.Query("match_all") == "true"

	// Parse pagination with defaults
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

	results, err := notesService.SearchNotes(c, searchOpts)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, results)
}

func CreateNoteHandler(c *gin.Context, notesService *usecase.NotesService) {
	var note model.Notes
	if err := c.ShouldBindJSON(&note); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	note.UserID = c.GetString("userID")
	if err := notesService.CreateNote(c, &note); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Note created successfully",
		"noteID":  note.ID,
	})
}

func UpdateNoteHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	var updates model.Notes
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	if err := notesService.UpdateNote(c, noteID, userID, &updates); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note updated successfully"})
}

func DeleteNoteHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := notesService.DeleteNote(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note deleted successfully"})
}

func ToggleFavoriteHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := notesService.ToggleFavorite(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note favorite status toggled successfully"})
}

func TogglePinHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := notesService.ToggleNotePin(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note pin status toggled successfully"})
}

func ArchiveNoteHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	if err := notesService.ArchiveNote(c, noteID, userID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Note archived successfully"})
}

func GetUserTagsHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")

	tags, err := notesService.GetUserTags(c, userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, tags)
}

func GetSearchSuggestionsHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")
	prefix := c.Query("prefix")

	suggestions, err := notesService.GetSearchSuggestions(userID, prefix)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, suggestions)
}

func GetUserNotesHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))

	notes, err := notesService.GetUserNotes(c, userID, limit)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, notes)
}

func GetArchivedNotesHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	notes, err := notesService.GetArchivedNotes(c, userID, page, pageSize)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, notes)
}

func UpdatePinPositionHandler(c *gin.Context, notesService *usecase.NotesService) {
	noteID := c.Param("id")
	userID := c.GetString("userID")

	var req struct {
		Position int `json:"position"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	if err := notesService.UpdatePinPosition(c, noteID, userID, req.Position); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Pin position updated successfully"})
}

func GetAllUserTagsHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")

	tags, err := notesService.GetAllUserTags(c, userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, tags)
}

func GetPinnedNotesHandler(c *gin.Context, notesService *usecase.NotesService) {
	userID := c.GetString("userID")

	pinnedNotes, err := notesService.NotesRepo.GetPinnedNotes(userID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, pinnedNotes)
}
