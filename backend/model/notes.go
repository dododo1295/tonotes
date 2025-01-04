package model

import (
	"time"
)

type Notes struct {
	UserID      string    `json:"user_id" bson:"user_id"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	NoteName    string    `bson:"note_name" json:"note_name"`
	NoteContent string    `bson:"note_content" json:"note_content"`
}
