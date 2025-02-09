package dto

import (
	"main/model"
	"time"
)

type TodoLink struct {
	Href   string `json:"href"`
	Method string `json:"method,omitempty"` // Optional: GET, POST, PUT, DELETE, PATCH
}

type TodoResponse struct {
	ID                string                  `json:"id"`
	TodoName          string                  `json:"todo_name"`
	Description       string                  `json:"description"`
	Complete          bool                    `json:"complete"`
	Priority          model.Priority          `json:"priority,omitempty"`
	Tags              []string                `json:"tags,omitempty"`
	DueDate           *time.Time              `json:"due_date,omitempty"`
	ReminderAt        *time.Time              `json:"reminder_at,omitempty"`
	IsRecurring       bool                    `json:"is_recurring"`
	RecurrencePattern model.RecurrencePattern `json:"recurrence_pattern,omitempty"`
	RecurrenceEndDate *time.Time              `json:"recurrence_end_date,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	TimeUntilDue      string                  `json:"time_until_due,omitempty"` // New computed field
	_links            map[string]TodoLink     `json:"_links,omitempty"`
}

// Convert model.Todos to TodoResponse
func ToTodoResponse(todo *model.Todo, links map[string]TodoLink) TodoResponse {
	response := TodoResponse{
		ID:                todo.TodoID,
		TodoName:          todo.TodoName,
		Description:       todo.Description,
		Complete:          todo.Complete,
		Priority:          todo.Priority,
		Tags:              todo.Tags,
		IsRecurring:       todo.IsRecurring,
		RecurrencePattern: todo.RecurrencePattern,
		CreatedAt:         todo.CreatedAt,
		UpdatedAt:         todo.UpdatedAt,
		_links:            links, // Set links
	}

	// Handle nullable time fields
	if !todo.DueDate.IsZero() {
		response.DueDate = &todo.DueDate
		// Calculate time until due
		if !todo.Complete {
			if todo.DueDate.Before(time.Now()) {
				response.TimeUntilDue = "Overdue"
			} else {
				response.TimeUntilDue = time.Until(todo.DueDate).Round(time.Hour).String()
			}
		}
	}

	if !todo.ReminderAt.IsZero() {
		response.ReminderAt = &todo.ReminderAt
	}

	if !todo.RecurrenceEndDate.IsZero() {
		response.RecurrenceEndDate = &todo.RecurrenceEndDate
	}

	return response
}

// Convert slice of model.Todos to slice of TodoResponse
func ToTodoResponses(todos []*model.Todo, getTodoLinks func(todo *model.Todo) map[string]TodoLink) []TodoResponse {
	responses := make([]TodoResponse, len(todos))
	for i, todo := range todos {
		responses[i] = ToTodoResponse(todo, getTodoLinks(todo))
	}
	return responses
}
