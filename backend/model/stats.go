package model

import "time"

type UserStats struct {
	NotesStats struct {
		Total     int            `json:"total"`
		Archived  int            `json:"archived"`
		Pinned    int            `json:"pinned"`
		TagCounts map[string]int `json:"tag_counts"`
	} `json:"notes_stats"`
	TodoStats struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Pending   int `json:"pending"`
	} `json:"todo_stats"`
	ActivityStats struct {
		LastActive     time.Time `json:"last_active"`
		AccountCreated time.Time `json:"account_created"`
		TotalSessions  int       `json:"total_sessions"`
	} `json:"activity_stats"`
}

type TodoStats struct {
	// Basic counts
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`

	// Priority based counts
	HighPriority   int `json:"high_priority"`
	MediumPriority int `json:"medium_priority"`
	LowPriority    int `json:"low_priority"`

	// Time based counts
	Overdue       int `json:"overdue"`
	DueToday      int `json:"due_today"`
	Upcoming      int `json:"upcoming"` // Due in next 7 days
	WithReminders int `json:"with_reminders"`
}
