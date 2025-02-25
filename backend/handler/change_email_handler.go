package handler

import (
	"log"
	"main/dto"
	"main/repository"
	"main/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

func ChangeEmailHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid email format")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	currentUser, err := userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching user %s: %v", userID, err)
		utils.InternalError(c, "Failed to fetch user details")
		return
	}

	if currentUser.Email == req.NewEmail {
		utils.BadRequest(c, "New email is same as current email")
		return
	}

	twoWeeks := 14 * 24 * time.Hour
	if time.Since(currentUser.LastEmailChange) < twoWeeks {
		nextAllowedChange := currentUser.LastEmailChange.Add(twoWeeks)
		utils.TooManyRequests(c, "Email can only be changed every 2 weeks", gin.H{
			"next_allowed_change": nextAllowedChange,
		})
		return
	}

	result, err := userRepo.UpdateUserEmail(userID.(string), req.NewEmail)
	if err != nil {
		log.Printf("Failed to update email for user %s: %v", userID, err)
		utils.InternalError(c, "Database error while updating email")
		return
	}

	if result == 0 {
		utils.NotFound(c, "User not found")
		return
	}

	log.Printf("Email changed successfully for user %s", userID)
	//Retrieve base URL
	baseURL := utils.GetBaseURL(c)

	//Generate Link
	links := map[string]dto.UserLink{
		"self":            {Href: baseURL + "/user", Method: http.MethodGet},
		"update-password": {Href: baseURL + "/user/password", Method: http.MethodPut},
		"delete":          {Href: baseURL + "/user", Method: http.MethodDelete},
	}

	updatedUser, err := userRepo.FindUser(userID.(string))
	if err != nil {
		log.Printf("Error fetching updated user %s: %v", userID, err)
		utils.InternalError(c, "Failed to fetch updated user details")
		return
	}

	userProfileResponse := dto.ToUserProfileResponse(updatedUser, links)

	utils.Success(c, userProfileResponse)
}
