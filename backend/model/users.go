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
	IsActive           bool      `bson:"is_active" json:"is_active"`
	TwoFactorSecret    string    `bson:"two_factor_secret,omitempty" json:"-"`
	TwoFactorEnabled   bool      `bson:"two_factor_enabled" json:"two_factor_enabled"`
	RecoveryCodes      []string  `bson:"recovery_codes,omitempty" json:"-"`
}

type LoginRequest struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	TwoFactorCode string `json:"two_factor_code,omitempty"` // Optional 2FA code
}

type UserProfile struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	// Add more if necessary
}
