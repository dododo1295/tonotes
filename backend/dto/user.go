package dto

import (
	"main/model"
	"time"
)

type UserLink struct {
	Href   string `json:"href"`
	Method string `json:"method,omitempty"` // Optional: GET, POST, PUT, DELETE, PATCH
}

type UserProfileResponse struct {
	Username  string              `json:"username"`
	Email     string              `json:"email"`
	CreatedAt time.Time           `json:"created_at"`
	Links     map[string]UserLink `json:"_links,omitempty"` // HAL UserLinks
}

func ToUserProfileResponse(user *model.User, links map[string]UserLink) UserProfileResponse {
	return UserProfileResponse{
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		Links:     links, // Set links
	}
}
