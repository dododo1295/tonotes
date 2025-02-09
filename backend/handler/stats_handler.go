package handler

import (
	"log"
	"main/model"
	"main/repository"
	"main/usecase"
	"main/utils"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	userRepo    *repository.UserRepo
	noteRepo    *repository.NoteRepo
	todoService *usecase.TodoService
	sessionRepo *repository.SessionRepo
}

func NewStatsHandler(
	userRepo *repository.UserRepo,
	noteRepo *repository.NoteRepo,
	todoService *usecase.TodoService,
	sessionRepo *repository.SessionRepo,
) *StatsHandler {
	return &StatsHandler{
		userRepo:    userRepo,
		noteRepo:    noteRepo,
		todoService: todoService,
		sessionRepo: sessionRepo,
	}
}

func (h *StatsHandler) GetUserStats(c *gin.Context) {
	ctx := c.Request.Context()
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	user, err := h.userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching user %s: %v", userID, err)
		utils.InternalError(c, "Failed to fetch user details")
		return
	}
	if user == nil {
		utils.NotFound(c, "User not found")
		return
	}

	var stats model.UserStats

	// Notes stats
	totalNotes, err := h.noteRepo.CountUserNotes(userID.(string))
	if err != nil {
		log.Printf("Error counting notes: %v", err)
		utils.InternalError(c, "Failed to count notes")
		return
	}
	stats.NotesStats.Total = totalNotes

	archivedNotes, err := h.noteRepo.GetArchivedNotes(userID.(string))
	if err != nil {
		log.Printf("Error getting archived notes: %v", err)
		utils.InternalError(c, "Failed to get archived notes")
		return
	}
	stats.NotesStats.Archived = len(archivedNotes)

	pinnedNotes, err := h.noteRepo.GetPinnedNotes(userID.(string))
	if err != nil {
		log.Printf("Error getting pinned notes: %v", err)
		utils.InternalError(c, "Failed to get pinned notes")
		return
	}
	stats.NotesStats.Pinned = len(pinnedNotes)

	tagCounts, err := h.noteRepo.CountNotesByTag(userID.(string))
	if err != nil {
		log.Printf("Error counting notes by tag: %v", err)
		utils.InternalError(c, "Failed to count notes by tag")
		return
	}
	stats.NotesStats.TagCounts = tagCounts

	// Todos stats
	completedTodos, err := h.todoService.GetCompletedTodos(ctx, userID.(string))
	if err != nil {
		log.Printf("Error getting completed todos: %v", err)
		utils.InternalError(c, "Failed to get completed todos")
		return
	}
	stats.TodoStats.Completed = len(completedTodos)

	pendingTodos, err := h.todoService.GetPendingTodos(ctx, userID.(string))
	if err != nil {
		log.Printf("Error getting pending todos: %v", err)
		utils.InternalError(c, "Failed to get pending todos")
		return
	}
	stats.TodoStats.Pending = len(pendingTodos)

	// Calculate total todos from completed and pending
	stats.TodoStats.Total = stats.TodoStats.Completed + stats.TodoStats.Pending

	// Sessions and activity stats
	sessions, err := h.sessionRepo.GetUserActiveSessions(userID.(string))
	if err != nil {
		log.Printf("Error getting sessions: %v", err)
		utils.InternalError(c, "Failed to get sessions")
		return
	}

	stats.ActivityStats.AccountCreated = user.CreatedAt
	stats.ActivityStats.TotalSessions = len(sessions)

	if len(sessions) > 0 {
		lastActive := sessions[0].LastActivityAt
		for _, session := range sessions {
			if session.LastActivityAt.After(lastActive) {
				lastActive = session.LastActivityAt
			}
		}
		stats.ActivityStats.LastActive = lastActive
	}

	utils.Success(c, gin.H{
		"stats": stats,
	})
}
