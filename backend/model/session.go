package model

import "time"

type Session struct {
	SessionID      string    `bson:"session_id" json:"session_id"`
	UserID         string    `bson:"user_id" json:"user_id"`
	DisplayName    string    `bson:"display_name" json:"display_name"`
	DeviceInfo     string    `bson:"device_info" json:"device_info"`
	Location       string    `bson:"location" json:"location"`
	IPAddress      string    `bson:"ip_address" json:"ip_address"`
	Protected      bool      `bson:"protected" json:"protected"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	ExpiresAt      time.Time `bson:"expires_at" json:"expires_at"`
	LastActivityAt time.Time `bson:"last_activity_at" json:"last_activity_at"`
	IsActive       bool      `bson:"is_active" json:"is_active"`
}

type Activity struct {
	Timestamp time.Time `bson:"timestamp"`
	Action    string    `bson:"action"`
	Location  string    `bson:"location"`
	IPAddress string    `bson:"ip_address"`
}
