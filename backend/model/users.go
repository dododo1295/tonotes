package model

type Users struct {
	UserID   string `json:"user_id" bson:"user_id"`   // Unique ID number
	Username string `bson:"username" json:"username"` // Username field
	Email    string `bson:"email" json:"email"`       // Email field
	Password string `bson:"password" json:"password"` // Hashed password field
}
