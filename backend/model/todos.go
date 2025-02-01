package model

import "time"

type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityMedium Priority = "MEDIUM"
	PriorityHigh   Priority = "HIGH"
)

type Todos struct {
	TodoID      string    `bson:"_id,omitempty" json:"id"`
	UserID      string    `bson:"user_id" json:"user_id"`
	TodoName    string    `bson:"todo_name" json:"todo_name" binding:"required"`
	Description string    `bson:"todo_description" json:"todo_description"`
	Complete    bool      `bson:"complete" json:"complete"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
	Tags        []string  `bson:"tags,omitempty" json:"tags,omitempty"`
	Priority    Priority  `bson:"priority" json:"priority"`
	DueDate     time.Time `bson:"due_date" json:"due_date"`
	ReminderAt  time.Time `bson:"reminder_at" json:"reminder_at"`
}
