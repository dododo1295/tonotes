package handler

import (
	"main/repository"
	"main/usecase"
	"main/utils"

	"github.com/gin-gonic/gin"
)

func GetUserProfileHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}

	userService := &usecase.UserService{
		UsersRepo: repository.GetUsersRepo(utils.MongoClient),
	}

	profile, err := userService.GetUserProfile(userID.(string))
	if err != nil {
		switch err.Error() {
		case "invalid user id":
			utils.BadRequest(c, "Invalid user id")
		case "user not found":
			utils.NotFound(c, "User not found")
		default:
			utils.InternalError(c, "Could not fetch user profile")
		}
		return
	}

	utils.Success(c, gin.H{
		"profile": profile,
	})
}
