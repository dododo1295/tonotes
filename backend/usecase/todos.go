package usecase

import (
	"context"
	"errors"
	"main/model"
	"main/repository"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TodosService struct {
	repo *repository.TodosRepo
}

func NewTodosService(repo *repository.TodosRepo) *TodosService {
	return &TodosService{repo: repo}
}

// Create Todo
func (svc *TodosService) CreateTodo(ctx context.Context, todo *model.Todos) error {
	if todo.UserID == "" {
		return errors.New("user ID is required")
	}
	if todo.TodoName == "" {
		return errors.New("todo name is required")
	}

	validatedTags, err := svc.validateTags(todo.Tags)
	if err != nil {
		return err
	}
	todo.Tags = validatedTags

	if err := validatePriority(todo.Priority); err != nil {
		return err
	}

	now := time.Now()
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = now
	}
	if todo.UpdatedAt.IsZero() {
		todo.UpdatedAt = now
	}

	if todo.TodoID == "" {
		todo.TodoID = uuid.New().String()
	}

	if !todo.DueDate.IsZero() && todo.DueDate.Before(now) {
		return errors.New("due date cannot be in the past")
	}
	if !todo.ReminderAt.IsZero() {
		if todo.ReminderAt.Before(now) {
			return errors.New("reminder time cannot be in the past")
		}
		if !todo.DueDate.IsZero() && todo.ReminderAt.After(todo.DueDate) {
			return errors.New("reminder time cannot be after due date")
		}
	}

	todo.Complete = false

	return svc.repo.CreateTodo(ctx, todo)
}

// Get Todos Count
func (svc *TodosService) CountUserTodos(ctx context.Context, userID string) (int, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(todos), nil
}

// Search Todos
func (svc *TodosService) SearchTodos(ctx context.Context, userID string, searchText string) ([]*model.Todos, error) {
	// Get all todos for the user
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If search text is empty, return empty result
	if searchText == "" {
		return []*model.Todos{}, nil
	}

	// Convert search text to lowercase for case-insensitive search
	searchText = strings.ToLower(searchText)
	var results []*model.Todos

	// Filter todos based on search text
	for _, todo := range todos {
		if strings.Contains(strings.ToLower(todo.TodoName), searchText) ||
			strings.Contains(strings.ToLower(todo.Description), searchText) {
			results = append(results, todo)
		}
	}

	return results, nil
}

// update Todos
func (svc *TodosService) UpdateTodo(ctx context.Context, todoID string, userID string, updates *model.Todos) error {
	// Check if todo exists
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("todo not found")
	}

	// Validate updated fields
	if updates.TodoName != "" {
		existing.TodoName = updates.TodoName
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}

	// Validate priority if it's being updated
	if updates.Priority != "" {
		if err := validatePriority(updates.Priority); err != nil {
			return err
		}
		existing.Priority = updates.Priority
	}

	// Validate tags if they're being updated
	if updates.Tags != nil {
		validatedTags, err := svc.validateTags(updates.Tags)
		if err != nil {
			return err
		}
		existing.Tags = validatedTags
	}

	// Update timestamps and status
	existing.UpdatedAt = time.Now()
	existing.Complete = updates.Complete

	// Validate and update dates if they're being changed
	if !updates.DueDate.IsZero() {
		if updates.DueDate.Before(time.Now()) {
			return errors.New("due date cannot be in the past")
		}
		existing.DueDate = updates.DueDate
	}

	if !updates.ReminderAt.IsZero() {
		if updates.ReminderAt.Before(time.Now()) {
			return errors.New("reminder time cannot be in the past")
		}
		if !existing.DueDate.IsZero() && updates.ReminderAt.After(existing.DueDate) {
			return errors.New("reminder time cannot be after due date")
		}
		existing.ReminderAt = updates.ReminderAt
	}

	// Update in repository
	return svc.repo.UpdateTodo(ctx, todoID, userID, existing)
}

// Get Todos by Priority
func (svc *TodosService) GetTodosByPriority(ctx context.Context, userID string, priority model.Priority) ([]*model.Todos, error) {
	// Validate priority first
	if err := validatePriority(priority); err != nil {
		return nil, err
	}

	// Get all todos for the user
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter todos by priority
	var filteredTodos []*model.Todos
	for _, todo := range todos {
		if todo.Priority == priority {
			filteredTodos = append(filteredTodos, todo)
		}
	}

	return filteredTodos, nil
}

// Get All Todos By Tag
func (svc *TodosService) GetTodosByTags(ctx context.Context, userID string, tags []string) ([]*model.Todos, error) {
	// Validate tags first
	validatedTags, err := svc.validateTags(tags)
	if err != nil {
		return nil, err
	}

	// Get all todos for the user
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If no tags provided, return empty result
	if len(validatedTags) == 0 {
		return []*model.Todos{}, nil
	}

	// Filter todos by tags
	var filteredTodos []*model.Todos
	for _, todo := range todos {
		if containsAnyTag(todo.Tags, validatedTags) {
			filteredTodos = append(filteredTodos, todo)
		}
	}

	return filteredTodos, nil
}

// Get User Tags
func (svc *TodosService) GetUserTags(ctx context.Context, userID string) ([]string, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Use map to store unique tags
	tagMap := make(map[string]bool)
	for _, todo := range todos {
		for _, tag := range todo.Tags {
			tagMap[tag] = true
		}
	}

	// Convert map keys to slice
	var uniqueTags []string
	for tag := range tagMap {
		uniqueTags = append(uniqueTags, tag)
	}

	return uniqueTags, nil
}

// Get Upcoming Todos
func (svc *TodosService) GetUpcomingTodos(ctx context.Context, userID string, days int) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	deadline := now.AddDate(0, 0, days)

	var upcomingTodos []*model.Todos
	for _, todo := range todos {
		if !todo.Complete && todo.DueDate.After(now) && todo.DueDate.Before(deadline) {
			upcomingTodos = append(upcomingTodos, todo)
		}
	}

	return upcomingTodos, nil
}

// Get Overdue Todos
func (svc *TodosService) GetOverdueTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var overdueTodos []*model.Todos
	for _, todo := range todos {
		if !todo.Complete && todo.DueDate.Before(now) {
			overdueTodos = append(overdueTodos, todo)
		}
	}

	return overdueTodos, nil
}

// Get Todos With Reminders
func (svc *TodosService) GetTodosWithReminders(ctx context.Context, userID string) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var todosWithReminders []*model.Todos
	for _, todo := range todos {
		if !todo.Complete && !todo.ReminderAt.IsZero() && todo.ReminderAt.After(now) {
			todosWithReminders = append(todosWithReminders, todo)
		}
	}

	return todosWithReminders, nil
}

// Toggle Todo Complete Status
func (svc *TodosService) ToggleTodoComplete(ctx context.Context, todoID string, userID string) error {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return err
	}

	var todoToUpdate *model.Todos
	for _, todo := range todos {
		if todo.TodoID == todoID {
			todoToUpdate = todo
			break
		}
	}

	if todoToUpdate == nil {
		return errors.New("todo not found")
	}

	// Toggle complete status
	updates := &model.Todos{
		Complete:  !todoToUpdate.Complete,
		UpdatedAt: time.Now(),
	}

	return svc.repo.UpdateTodo(ctx, todoID, userID, updates)
}

// Get Completed Todos
func (svc *TodosService) GetCompletedTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	var completedTodos []*model.Todos
	for _, todo := range todos {
		if todo.Complete {
			completedTodos = append(completedTodos, todo)
		}
	}

	return completedTodos, nil
}

// Get Pending Todos
func (svc *TodosService) GetPendingTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	var pendingTodos []*model.Todos
	for _, todo := range todos {
		if !todo.Complete {
			pendingTodos = append(pendingTodos, todo)
		}
	}

	return pendingTodos, nil
}

// helper
func validatePriority(p model.Priority) error {
	switch p {
	case model.PriorityLow, model.PriorityMedium, model.PriorityHigh:
		return nil
	case "": // empty priority is valid
		return nil
	default:
		return errors.New("invalid priority level")
	}
}

func (svc *TodosService) validateTags(tags []string) ([]string, error) {
	if tags == nil || len(tags) == 0 {
		return nil, nil
	}
	var validTags []string
	for _, tag := range tags {
		if tag != "" {
			validTags = append(validTags, tag)
		}
	}
	if len(validTags) > 5 {
		return nil, errors.New("cannot exceed 5 tags per todo")
	}

	for _, tag := range validTags {
		if len(tag) > 20 {
			return nil, errors.New("tag cannot exceed 20 characters")
		}
	}

	return validTags, nil
}

func containsAnyTag(todoTags []string, searchTags []string) bool {
	for _, searchTag := range searchTags {
		for _, todoTag := range todoTags {
			if strings.EqualFold(searchTag, todoTag) { // case-insensitive comparison
				return true
			}
		}
	}
	return false
}
