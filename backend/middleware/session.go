package middleware

import (
	"fmt"
	"main/model"
	"main/repository"
	"main/utils"
	"time"

	"github.com/gin-gonic/gin"
    "github.com/google/uuid"	
)

type SessionRepository interface {
	CreateSession(*model.Session) error
	GetSession(string) (*model.Session, error)
	UpdateSession(*model.Session) error
	DeleteSession(string) error
	CountActiveSessions(string) (int, error)
	EndLeastActiveSession(string) error
}

func SessionMiddleware(sessionRepo *repository.SessionRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			c.Next()
			return
		}

		session, err := sessionRepo.GetSession(sessionID)
		if err != nil || !session.IsActive {
			c.SetCookie("session_id", "", -1, "/", "", true, true)
			c.Next()
			return
		}

		// Check for inactivity timeout (48 hours)
		if time.Since(session.LastActivityAt) > 48*time.Hour {
			session.IsActive = false
			sessionRepo.UpdateSession(session)
			c.SetCookie("session_id", "", -1, "/", "", true, true)
			c.Next()
			return
		}

		// Update last activity time
		session.LastActivityAt = time.Now()
		sessionRepo.UpdateSession(session)

		c.Set("session", session)
		c.Next()
	}
}
func CreateSession(c *gin.Context, userID string, sessionRepo *repository.SessionRepo) error {
	// Generate session name from user agent
	userAgent := c.Request.UserAgent()
	browser, os, device := utils.ParseUserAgent(userAgent)

	// Create display name
	displayName := utils.GenerateSessionName(userAgent, "") // Empty string for location for now

	session := &model.Session{
		SessionID:      uuid.New().String(),
		UserID:         userID,
		DisplayName:    displayName,
		DeviceInfo:     fmt.Sprintf("%s on %s (%s)", browser, os, device),
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
