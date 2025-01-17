package model

import (
	"time"
)

type Notes struct {
	ID         string    `bson:"_id,omitempty" json:"id"`
	UserID     string    `bson:"user_id" json:"user_id"`
	Title      string    `bson:"title" json:"title" binding:"required"`
	Content    string    `bson:"content" json:"content"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
	Tags       []string  `bson:"tags,omitempty" json:"tags,omitempty"`
	IsArchived bool      `bson:"is_archived" json:"is_archived"`
}
