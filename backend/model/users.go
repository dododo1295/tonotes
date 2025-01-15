package model

import "time"

type User struct {
	UserID             string    `bson:"user_id" json:"user_id"`                                   // Unique ID number
	Username           string    `bson:"username" json:"username" binding:"required,min=4,max=20"` // Username field
	Email              string    `bson:"email" json:"email" binding:"required,email"`              // Updated this line
	Password           string    `bson:"password" json:"password" binding:"required,password"`     // Keep custom validator
	CreatedAt          time.Time `bson:"createdAt" json:"createdAt"`                               // Time created for account life
	LastEmailChange    time.Time `bson:"lastEmailChange" json:"lastEmailChange"`                   // Track last email change for rate limiting
	LastPasswordChange time.Time `bson:"lastPasswordChange" json:"lastPasswordChange"`             // Track last password change for rate limiting
}
