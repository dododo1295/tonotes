package handler

import (
	"fmt"
	"main/model"
	"main/repository"
	"main/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetActiveSessions(c *gin.Context, sessionRepo *repository.SessionRepo) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	sessions, err := sessionRepo.GetUserActiveSessions(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch sessions")
		return
	}

	// Format session data for response
	formattedSessions := make([]map[string]interface{}, 0)
	for _, session := range sessions {
		formattedSessions = append(formattedSessions, map[string]interface{}{
			"session_id":       session.SessionID,
			"display_name":     session.DisplayName,
			"device_info":      session.DeviceInfo,
			"ip_address":       session.IPAddress,
			"location":         session.Location,
			"created_at":       session.CreatedAt,
			"last_activity_at": session.LastActivityAt,
			"is_current":       session.SessionID == c.GetString("session_id"),
		})
	}

	utils.Success(c, gin.H{
		"sessions": formattedSessions,
		"count":    len(sessions),
	})
}

func LogoutAllSessions(c *gin.Context, sessionRepo *repository.SessionRepo) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	// End all sessions for the user
	if err := sessionRepo.EndAllUserSessions(userID.(string)); err != nil {
		utils.InternalError(c, "Failed to end all sessions")
		return
	}

	// Clear current session cookie
	c.SetCookie("session_id", "", -1, "/", "", true, true)

	utils.Success(c, gin.H{
		"message": "Successfully logged out of all sessions",
	})
}

func LogoutSession(c *gin.Context, sessionRepo *repository.SessionRepo) {
	sessionID := c.Param("session_id")
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	session, err := sessionRepo.GetSession(sessionID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch session")
		return
	}

	if session == nil || session.UserID != userID {
		utils.Forbidden(c, "Access denied")
		return
	}

	if err := sessionRepo.DeleteSession(sessionID); err != nil {
		utils.InternalError(c, "Failed to delete session")
		return
	}

	utils.Success(c, gin.H{
		"message": "Successfully logged out of the session",
	})
}

func GetSessionDetails(c *gin.Context, sessionRepo *repository.SessionRepo) {
	sessionID := c.Param("session_id")
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	session, err := sessionRepo.GetSession(sessionID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch session")
		return
	}

	if session == nil || session.UserID != userID {
		utils.Forbidden(c, "Access denied")
		return
	}

	utils.Success(c, gin.H{
		"session": session,
	})
}

func UpdateSession(c *gin.Context, sessionRepo *repository.SessionRepo) {
	sessionID := c.Param("session_id")
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	var updateData struct {
		Protected bool `json:"protected"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		utils.BadRequest(c, "Invalid request data")
		return
	}

	session, err := sessionRepo.GetSession(sessionID)
	if err != nil {
		utils.InternalError(c, "Failed to fetch session")
		return
	}

	if session == nil || session.UserID != userID {
		utils.Forbidden(c, "Access denied")
		return
	}

	session.Protected = updateData.Protected
	if err := sessionRepo.UpdateSession(session); err != nil {
		utils.InternalError(c, "Failed to update session")
		return
	}

	utils.Success(c, gin.H{
		"message": "Session updated successfully",
	})
}

func CreateSession(c *gin.Context, userID string, sessionRepo *repository.SessionRepo) error {
	// Generate session name from user agent
	userAgent := c.Request.UserAgent()
	browser, os, device := utils.ParseUserAgent(userAgent)

	// Get location from IP
	location, err := utils.GetLocationFromIP(c.ClientIP())
	if err != nil {
		location = "Unknown Location"
	}

	// Create display name
	displayName := utils.GenerateSessionName(userAgent, location)

	session := &model.Session{
		SessionID:      uuid.New().String(),
		UserID:         userID,
		DisplayName:    displayName,
		DeviceInfo:     fmt.Sprintf("%s on %s (%s)", browser, os, device),
		Location:       location,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		LastActivityAt: time.Now(),
		IPAddress:      c.ClientIP(),
		IsActive:       true,
	}

	if err := sessionRepo.CreateSession(session); err != nil {
		return err
	}

	c.SetCookie(
		"session_id",
		session.SessionID,
		int(24*time.Hour.Seconds()),
		"/",
		"",
		true,
		true,
	)

	return nil
}
