package handler

import (
	"main/dto"
	"main/repository"
	"main/usecase"
	"main/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetUserProfileHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}

	userService := &usecase.UserService{
		UsersRepo: repository.GetUserRepo(utils.MongoClient),
	}

	user, err := userService.UsersRepo.FindUser(userID.(string)) // Changed from GetUserProfile to directly fetch the User model.  This assumes the UserProfile is simply derived from the User.
	if err != nil {
		switch err.Error() {
		case "invalid user id":
			utils.BadRequest(c, "Invalid user id")
		case "user not found":
			utils.NotFound(c, "User not found")
		default:
			utils.InternalError(c, "Could not fetch user details")
		}
		return
	}
	if user == nil {
		utils.NotFound(c, "User not found")
		return
	}

	baseURL := utils.GetBaseURL(c)
	links := map[string]dto.UserLink{
		"self":            {Href: baseURL + "/user", Method: http.MethodGet},          // URL to get the user profile
		"update-email":    {Href: baseURL + "/user/email", Method: http.MethodPut},    // URL to update the email
		"update-password": {Href: baseURL + "/user/password", Method: http.MethodPut}, // URL to update the password
		"delete":          {Href: baseURL + "/user", Method: http.MethodDelete},
	}

	userProfileResponse := dto.ToUserProfileResponse(user, links)
	utils.Success(c, userProfileResponse) // Send structured response
}
