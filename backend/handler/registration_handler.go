package handler

import (
	"main/dto"
	"main/model"
	"main/repository"
	"main/utils"
	"net/http"
	"time"

	"main/services"
	"main/usecase"

	"fmt"

	"github.com/gin-gonic/gin"
)

func RegistrationHandler(c *gin.Context) {
	// Start timing the request
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		utils.HTTPRequestDuration.WithLabelValues("POST", "/register").Observe(duration)
		utils.RequestDistribution.WithLabelValues("POST", "/register", fmt.Sprintf("%d", c.Writer.Status())).Observe(duration)
	}()

	var user model.User

	// Bind JSON and validate request
	if err := c.ShouldBindJSON(&user); err != nil {
		utils.TrackError("registration", "validation_error")
		utils.BadRequest(c, "invalid request")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	userService := &usecase.UserService{
		UsersRepo: userRepo,
	}

	// Track database operation timing
	dbTimer := utils.TrackDBOperation("create", "users")
	err := userService.CreateUser(c, &user)
	dbTimer.ObserveDuration()

	if err != nil {
		if err.Error() == "username already exists" {
			utils.TrackError("registration", "duplicate_username")
			utils.Conflict(c, "username already exists")
			return
		}
		utils.TrackError("registration", "creation_failed")
		utils.BadRequest(c, "invalid request")
		return
	}

	// Generate access token
	token, err := services.GenerateToken(user.UserID)
	if err != nil {
		utils.TrackError("registration", "token_generation")
		utils.InternalError(c, "failed to generate token")
		return
	}
	utils.TokenUsage.WithLabelValues("access", "generated").Inc()

	// Generate refresh token
	refreshToken, err := services.GenerateRefreshToken(user.UserID)
	if err != nil {
		utils.TrackError("registration", "refresh_token_generation")
		utils.InternalError(c, "failed to generate refresh token")
		return
	}
	utils.TokenUsage.WithLabelValues("refresh", "generated").Inc()

	baseURL := utils.GetBaseURL(c)

	// Create the links map
	links := map[string]dto.UserLink{
		"self": {Href: baseURL + "/user", Method: http.MethodGet}, // Corrected path
	}

	userProfileResponse := dto.ToUserProfileResponse(&user, links)
	utils.TrackRegistration()
	utils.UserGrowthRate.Inc()
	utils.TrackUserActivity(user.UserID)
	utils.Created(c, gin.H{
		"message": "user registered successfully",
		"profile": userProfileResponse, // Now returning the DTO with links
		"token":   token,
		"refresh": refreshToken,
	})
}
