package model

import "time"

type Todos struct {
	ID              string    `bson:"_id,omitempty" json:"id"`
	UserID          string    `bson:"user_id" json:"user_id"`
	TodoName        string    `bson:"todo_name" json:"todo_name" binding:"required"`
	TodoDescription string    `bson:"todo_description" json:"todo_description"`
	Complete        bool      `bson:"complete" json:"complete"`
	CreatedAt       time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time `bson:"updated_at" json:"updated_at"`
	Tags            []string  `bson:"tags,omitempty" json:"tags,omitempty"`
}
