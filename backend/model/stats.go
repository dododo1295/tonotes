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
