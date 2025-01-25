package model

import "time"

type User struct {
	UserID             string    `bson:"user_id" json:"user_id"`
	Username           string    `bson:"username" json:"username" binding:"required,min=4,max=20"`
	Email              string    `bson:"email" json:"email" binding:"required,email"`
	Password           string    `bson:"password" json:"password" binding:"required,password"`
	CreatedAt          time.Time `bson:"createdAt" json:"createdAt"`
	LastEmailChange    time.Time `bson:"lastEmailChange" json:"lastEmailChange"`
	LastPasswordChange time.Time `bson:"lastPasswordChange" json:"lastPasswordChange"`
	IsActive           bool      `bson:"is_active" json:"is_active"` // Add this field
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserProfile struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	// Add more if necessary
}
