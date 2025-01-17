package middleware

import (
	"main/model"
	"main/repository"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func SessionMiddleware(sessionRepo *repository.SessionRepo) gin.HandlerFunc {
    return func(c *gin.Context) {
        sessionID, err := c.Cookie("session_id")
        if err != nil {
            c.Next()
            return
        }

        session, err := sessionRepo.GetSession(sessionID)
        if err != nil || !session.IsActive || time.Now().After(session.ExpiresAt) {
            c.SetCookie("session_id", "", -1, "/", "", true, true)
            c.Next()
            return
        }

        // Update last activity
        session.LastActivityAt = time.Now()
        sessionRepo.UpdateSession(session)

        c.Set("session", session)
        c.Next()
    }
}

func CreateSession(c *gin.Context, userID string, sessionRepo *repository.SessionRepo) error {
    session := &model.Session{
        SessionID:      uuid.New().String(),
        UserID:         userID,
        CreatedAt:      time.Now(),
        ExpiresAt:      time.Now().Add(24 * time.Hour),
        LastActivityAt: time.Now(),
        DeviceInfo:     c.Request.UserAgent(),
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
