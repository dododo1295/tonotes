package model

import "time"

type Priority string
type RecurrencePattern string

const (
	PriorityLow    Priority = "LOW"
	PriorityMedium Priority = "MEDIUM"
	PriorityHigh   Priority = "HIGH"

	RecurrenceDaily   RecurrencePattern = "DAILY"
	RecurrenceWeekly  RecurrencePattern = "WEEKLY"
	RecurrenceMonthly RecurrencePattern = "MONTHLY"
	RecurrenceYearly  RecurrencePattern = "YEARLY"
)

type Todo struct {
	TodoID            string            `bson:"_id,omitempty" json:"id"`
	UserID            string            `bson:"user_id" json:"user_id"`
	TodoName          string            `bson:"todo_name" json:"todo_name" binding:"required"`
	Description       string            `bson:"todo_description" json:"description"`
	Complete          bool              `bson:"complete" json:"complete"`
	CreatedAt         time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time         `bson:"updated_at" json:"updated_at"`
	Tags              []string          `bson:"tags,omitempty" json:"tags,omitempty"`
	Priority          Priority          `bson:"priority,omitempty" json:"priority,omitempty"`
	DueDate           time.Time         `bson:"due_date,omitempty" json:"due_date,omitempty"`
	ReminderAt        time.Time         `bson:"reminder_at,omitempty" json:"reminder_at,omitempty"`
	IsRecurring       bool              `bson:"is_recurring,omitempty" json:"is_recurring,omitempty"`
	RecurrencePattern RecurrencePattern `bson:"recurrence_pattern,omitempty" json:"recurrence_pattern,omitempty"`
	RecurrenceEndDate time.Time         `bson:"recurrence_end_date,omitempty" json:"recurrence_end_date,omitempty"`
}
