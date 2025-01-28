package handler

import (
	"fmt"
	"log"
	"main/middleware"
	"main/model"
	"main/repository"
	"main/services"
	"main/usecase"
	"main/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

const MaxActiveSessions = 5

func LoginHandler(c *gin.Context, sessionRepo *repository.SessionRepo) {
	// Start timing the request
	start := time.Now()
	defer func() {
		// Track request duration and distribution
		duration := time.Since(start).Seconds()
		utils.HTTPRequestDuration.WithLabelValues("POST", "/login").Observe(duration)
		utils.RequestDistribution.WithLabelValues("/login", fmt.Sprintf("%d", c.Writer.Status())).Observe(duration)
	}()

	var loginReq model.LoginRequest

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		utils.TrackError("auth", "invalid_request")
		utils.TrackAuthAttempt("failure", "validation")
		utils.BadRequest(c, "Invalid Request")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	// Track DB operation timing
	dbTimer := utils.TrackDBOperation("find", "users")
	user, err := userService.FindUserByUsername(loginReq.Username)
	dbTimer.ObserveDuration()

	if err != nil {
		utils.TrackError("auth", "user_lookup")
		utils.TrackAuthAttempt("failure", "invalid_username")
		utils.Unauthorized(c, "Invalid username")
		return
	}

	if user == nil {
		utils.TrackError("auth", "user_not_found")
		utils.TrackAuthAttempt("failure", "user_not_found")
		utils.Unauthorized(c, "Invalid username")
		return
	}

	// Verify password
	checkPassword, err := services.VerifyPassword(user.Password, loginReq.Password)
	if err != nil {
		utils.TrackError("auth", "password_verification")
		utils.TrackAuthAttempt("failure", "password_verification_error")
		utils.Unauthorized(c, "Incorrect Password")
		return
	}
	if !checkPassword {
		utils.TrackAuthAttempt("failure", "invalid_password")
		utils.Unauthorized(c, "Incorrect Password")
		return
	}

	// 2FA Handling with metrics
	if user.TwoFactorEnabled {
		if loginReq.TwoFactorCode == "" {
			utils.TrackAuthAttempt("pending", "2fa_required")
			utils.Success(c, gin.H{
				"requires_2fa": true,
				"message":      "2FA code required",
				"user_id":      user.UserID,
			})
			return
		}

		valid := totp.Validate(loginReq.TwoFactorCode, user.TwoFactorSecret)
		if !valid {
			utils.TrackAuthAttempt("failure", "invalid_2fa")
			utils.TrackError("auth", "invalid_2fa_code")
			utils.Unauthorized(c, "Invalid 2FA code")
			return
		}
		utils.TrackAuthAttempt("success", "2fa")
	}

	// Session management with metrics
	activeCount, err := sessionRepo.CountActiveSessions(user.UserID)
	if err != nil {
		utils.TrackError("session", "count_check")
		utils.InternalError(c, "Failed to check session count")
		return
	}

	var notice string
	if activeCount >= MaxActiveSessions {
		if err := sessionRepo.EndLeastActiveSession(user.UserID); err != nil {
			utils.TrackError("session", "session_cleanup")
			utils.InternalError(c, "Failed to manage sessions")
			return
		}
		notice = "Logged out of least active session due to session limit"
		log.Printf("Ended least active session for user %s due to session limit", user.UserID)
		utils.TrackError("session", "session_limit_reached")
	}

	// Token generation with metrics
	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		utils.TrackError("auth", "token_generation")
		utils.InternalError(c, "Failed to generate token")
		return
	}
	utils.TokenUsage.WithLabelValues("access", "generated").Inc()

	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		utils.TrackError("auth", "refresh_token_generation")
		utils.InternalError(c, "Failed to generate refresh token")
		return
	}
	utils.TokenUsage.WithLabelValues("refresh", "generated").Inc()

	// Session creation with metrics
	if err := middleware.CreateSession(c, user.UserID, sessionRepo); err != nil {
		utils.TrackError("session", "creation")
		utils.InternalError(c, "Failed to create session")
		return
	}

	// Track successful login and user activity
	utils.TrackAuthAttempt("success", "login")
	utils.TrackUserActivity(user.UserID)

	// Prepare response
	response := gin.H{
		"message": "Login successful",
		"token":   token,
		"refresh": refreshToken,
		"user": gin.H{
			"id":       user.UserID,
			"username": user.Username,
			"email":    user.Email,
		},
	}

	if notice != "" {
		response["notice"] = notice
	}

	utils.Success(c, response)
}
