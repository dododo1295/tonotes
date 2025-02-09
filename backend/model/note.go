package model

import (
	"time"
)

type Note struct {
	ID             string    `bson:"_id,omitempty" json:"id"`
	UserID         string    `bson:"user_id" json:"user_id"`
	Title          string    `bson:"title" json:"title" binding:"required"`
	Content        string    `bson:"content" json:"content" binding:"required"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
	Tags           []string  `bson:"tags,omitempty" json:"tags,omitempty"`
	IsPinned       bool      `bson:"is_pinned" json:"is_pinned"`
	IsArchived     bool      `bson:"is_archived" json:"is_archived"`
	PinnedPosition int       `bson:"pinned_position,omitempty" json:"pinned_position,omitempty"`
	SearchScore    float64   `bson:"score,omitempty" json:"search_score,omitempty"`
}
