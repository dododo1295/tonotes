package handler

import (
	"fmt"
	"main/model"
	"main/repository"
	"main/services"
	"strings"
	"time"

	"main/utils"

	"github.com/gin-gonic/gin"
)

func LogoutHandler(c *gin.Context, sessionRepo *repository.SessionRepo) {
	// Start timing the request
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		utils.HTTPRequestDuration.WithLabelValues("POST", "/logout").Observe(duration)
		utils.RequestDistribution.WithLabelValues("/logout", fmt.Sprintf("%d", c.Writer.Status())).Observe(duration)
	}()

	// Get the access token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		utils.TrackError("auth", "invalid_token")
		utils.TrackAuthAttempt("failure", "logout_invalid_token")
		utils.Unauthorized(c, "Invalid access token")
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Get refresh token
	refreshToken := c.GetHeader("Refresh-Token")
	if refreshToken == "" {
		utils.TrackError("auth", "missing_refresh_token")
		utils.TrackAuthAttempt("failure", "logout_missing_refresh")
		utils.BadRequest(c, "Missing refresh token")
		return
	}

	// Blacklist tokens
	if err := services.BlacklistTokens(accessToken, refreshToken); err != nil {
		utils.TrackError("auth", "token_blacklist_failed")
		utils.TrackAuthAttempt("failure", "logout_blacklist_failed")
		utils.InternalError(c, "Failed to blacklist tokens")
		return
	}

	// Track token invalidation
	utils.TokenUsage.WithLabelValues("access", "blacklisted").Inc()
	utils.TokenUsage.WithLabelValues("refresh", "blacklisted").Inc()

	// Update session
	if session, exists := c.Get("session"); exists {
		currentSession := session.(*model.Session)
		currentSession.IsActive = false

		// Track session operation timing
		dbTimer := utils.TrackDBOperation("update", "sessions")
		err := sessionRepo.UpdateSession(currentSession)
		dbTimer.ObserveDuration()

		if err != nil {
			utils.TrackError("session", "update_failed")
			utils.InternalError(c, "Failed to end session")
			return
		}
	}

	// Track successful logout
	utils.TrackAuthAttempt("success", "logout")

	utils.Success(c, gin.H{
		"message": "Successfully logged out",
	})
}
