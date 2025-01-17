package model

import "time"

type Session struct {
	SessionID      string    `bson:"session_id" json:"session_id"`
	UserID         string    `bson:"user_id" json:"user_id"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	ExpiresAt      time.Time `bson:"expires_at" json:"expires_at"`
	LastActivityAt time.Time `bson:"last_activity_at" json:"last_activity_at"`
	DeviceInfo     string    `bson:"device_info" json:"device_info"`
	IPAddress      string    `bson:"ip_address" json:"ip_address"`
	IsActive       bool      `bson:"is_active" json:"is_active"`
}
