package model

import "time"

type User struct {
	UserID       string    `bson:"user_id" json:"user_id"`                                     // Unique ID number
	Username     string    `bson:"username" json:"username" validate:"required, min=4 max=20"` // Username field
	Email        string    `bson:"email" json:"email" validate:"email, required" `             // Email field
	Password     string    `bson:"password" json:"password" validate:"required, min=6"`        // Hashed password field
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`                                 // Time created for account life
	Token        string    `bson:"token" json:"token"`                                         // JWT Token
	RefreshToken string    `bson:"refresh_token" json:"refresh_token"`                         // refreshed token
}
