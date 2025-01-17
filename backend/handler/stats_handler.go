package handler

import (
	"log"
	"main/model"
	"main/repository"
	"main/utils"

	"github.com/gin-gonic/gin"
)

func GetUserStatsHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	// Initialize repositories
	userRepo := repository.GetUsersRepo(utils.MongoClient)
	notesRepo := repository.GetNotesRepo(utils.MongoClient)
	todoRepo := repository.GetTodosRepo(utils.MongoClient)
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)

	// Get user info
	user, err := userRepo.FindUser(userID.(string))
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

	// Collect Notes Statistics
	totalNotes, err := notesRepo.CountUserNotes(userID.(string))
	if err != nil {
		log.Printf("Error counting notes: %v", err)
		utils.InternalError(c, "Failed to count notes")
		return
	}
	stats.NotesStats.Total = totalNotes

	archivedNotes, err := notesRepo.GetArchivedNotes(userID.(string))
	if err != nil {
		log.Printf("Error getting archived notes: %v", err)
		utils.InternalError(c, "Failed to get archived notes")
		return
	}
	stats.NotesStats.Archived = len(archivedNotes)

	pinnedNotes, err := notesRepo.GetPinnedNotes(userID.(string))
	if err != nil {
		log.Printf("Error getting pinned notes: %v", err)
		utils.InternalError(c, "Failed to get pinned notes")
		return
	}
	stats.NotesStats.Pinned = len(pinnedNotes)

	// Get tag statistics
	tagCounts, err := notesRepo.CountNotesByTag(userID.(string))
	if err != nil {
		log.Printf("Error counting notes by tag: %v", err)
		utils.InternalError(c, "Failed to count notes by tag")
		return
	}
	stats.NotesStats.TagCounts = tagCounts

	// Collect Todo Statistics
	totalTodos, err := todoRepo.CountUserTodos(userID.(string))
	if err != nil {
		log.Printf("Error counting todos: %v", err)
		utils.InternalError(c, "Failed to count todos")
		return
	}
	stats.TodoStats.Total = totalTodos

	completedTodos, err := todoRepo.GetCompletedTodos(userID.(string))
	if err != nil {
		log.Printf("Error getting completed todos: %v", err)
		utils.InternalError(c, "Failed to get completed todos")
		return
	}
	stats.TodoStats.Completed = len(completedTodos)

	pendingTodos, err := todoRepo.GetPendingTodos(userID.(string))
	if err != nil {
		log.Printf("Error getting pending todos: %v", err)
		utils.InternalError(c, "Failed to get pending todos")
		return
	}
	stats.TodoStats.Pending = len(pendingTodos)

	// Collect Activity Statistics
	sessions, err := sessionRepo.GetUserActiveSessions(userID.(string))
	if err != nil {
		log.Printf("Error getting sessions: %v", err)
		utils.InternalError(c, "Failed to get sessions")
		return
	}

	stats.ActivityStats.AccountCreated = user.CreatedAt
	stats.ActivityStats.TotalSessions = len(sessions)

	// Find most recent activity
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
