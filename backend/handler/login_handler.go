package handler

import (
	"log"
	"main/middleware"
	"main/model"
	"main/repository"
	"main/services"
	"main/usecase"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

const MaxActiveSessions = 5

func LoginHandler(c *gin.Context, sessionRepo *repository.SessionRepo) {
	var loginReq model.LoginRequest

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		utils.BadRequest(c, "Invalid Request")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	user, err := userService.FindUserByUsername(loginReq.Username)
	if err != nil {
		utils.Unauthorized(c, "Invalid username")
		return
	}

	if user == nil {
		utils.Unauthorized(c, "Invalid username")
		return
	}

	// Verify password
	checkPassword, err := services.VerifyPassword(user.Password, loginReq.Password)
	if err != nil {
		utils.Unauthorized(c, "Incorrect Password")
		return
	}
	if !checkPassword {
		utils.Unauthorized(c, "Incorrect Password")
		return
	}

	// Check if 2FA is enabled
	if user.TwoFactorEnabled {
		// If 2FA is enabled but no code provided
		if loginReq.TwoFactorCode == "" {
			utils.Success(c, gin.H{
				"requires_2fa": true,
				"message":      "2FA code required",
				"user_id":      user.UserID, // Optionally include user ID for subsequent 2FA verification
			})
			return
		}

		// Verify 2FA code
		valid := totp.Validate(loginReq.TwoFactorCode, user.TwoFactorSecret)
		if !valid {
			utils.Unauthorized(c, "Invalid 2FA code")
			return
		}
	}

	// Check active session count
	activeCount, err := sessionRepo.CountActiveSessions(user.UserID)
	if err != nil {
		utils.InternalError(c, "Failed to check session count")
		return
	}

	var notice string
	if activeCount >= MaxActiveSessions {
		// End the least active session instead of rejecting the login
		if err := sessionRepo.EndLeastActiveSession(user.UserID); err != nil {
			utils.InternalError(c, "Failed to manage sessions")
			return
		}
		notice = "Logged out of least active session due to session limit"
		log.Printf("Ended least active session for user %s due to session limit", user.UserID)
	}

	// Generate tokens
	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		utils.InternalError(c, "Failed to generate token")
		return
	}

	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		utils.InternalError(c, "Failed to generate refresh token")
		return
	}

	// Create new session
	if err := middleware.CreateSession(c, user.UserID, sessionRepo); err != nil {
		utils.InternalError(c, "Failed to create session")
		return
	}

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

	// Add notice if a session was ended
	if notice != "" {
		response["notice"] = notice
	}

	// Return success response
	utils.Success(c, response)
}
