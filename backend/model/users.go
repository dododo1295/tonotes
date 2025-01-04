package model

import "time"

type User struct {
	UserID    string    `bson:"user_id" json:"user_id"`     // Unique ID number
	Username  string    `bson:"username" json:"username"`   // Username field
	Email     string    `bson:"email" json:"email"`         // Email field
	Password  string    `bson:"password" json:"password"`   // Hashed password field
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"` // Time created for account life
}
