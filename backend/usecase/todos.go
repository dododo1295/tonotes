package usecase

import (
	"context"
	"errors"
	"main/model"
	"main/repository"
	"sort"
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

// Get the user's todos
func (svc *TodosService) GetUserTodos(ctx context.Context, userID string) ([]*model.Todos, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Sort todos by priority and due date
	sort.Slice(todos, func(i, j int) bool {
		// First sort by completion status (incomplete first)
		if todos[i].Complete != todos[j].Complete {
			return !todos[i].Complete
		}

		// Then by overdue status for incomplete todos
		if !todos[i].Complete && !todos[j].Complete {
			iOverdue := !todos[i].DueDate.IsZero() && todos[i].DueDate.Before(time.Now())
			jOverdue := !todos[j].DueDate.IsZero() && todos[j].DueDate.Before(time.Now())
			if iOverdue != jOverdue {
				return iOverdue // Show overdue items first
			}
		}

		// Then by priority
		if todos[i].Priority != todos[j].Priority {
			return getPriorityWeight(todos[i].Priority) > getPriorityWeight(todos[j].Priority)
		}

		// Then by due date (if exists)
		if !todos[i].DueDate.IsZero() && !todos[j].DueDate.IsZero() {
			return todos[i].DueDate.Before(todos[j].DueDate)
		}

		// Finally by creation date
		return todos[i].CreatedAt.Before(todos[j].CreatedAt)
	})

	return todos, nil
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

	if todo.IsRecurring {
		switch todo.RecurrencePattern {
		case model.RecurrenceDaily, model.RecurrenceWeekly, model.RecurrenceMonthly, model.RecurrenceYearly:
		default:
			return errors.New("invalid recurrence pattern")
		}

		if todo.DueDate.IsZero() {
			return errors.New("due date is required for recurring todos")
		}
	}

	if !todo.Complete {
		todo.Complete = false
	}

	return svc.repo.CreateTodo(ctx, todo)
}

// Delete todos
func (svc *TodosService) DeleteTodo(ctx context.Context, todoID string, userID string) error {
	// Verify todo exists and belongs to user
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("todo not found")
	}

	if existing.Complete && existing.DueDate.After(time.Now()) {
		return errors.New("cannot delete completed todo with future due date")
	}

	return svc.repo.DeleteTodo(ctx, todoID, userID)
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
func (svc *TodosService) UpdateTodo(ctx context.Context, todoID string, userID string, updates *model.Todos) (*model.Todos, error) {
	// Check if todo exists
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
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
			return nil, err
		}
		existing.Priority = updates.Priority
	}

	// Validate tags if they're being updated
	if updates.Tags != nil {
		validatedTags, err := svc.validateTags(updates.Tags)
		if err != nil {
			return nil, err
		}
		existing.Tags = validatedTags
	}

	// Update timestamps and status
	existing.UpdatedAt = time.Now()
	existing.Complete = updates.Complete

	// Validate and update dates if they're being changed
	if !updates.DueDate.IsZero() {
		if updates.DueDate.Before(time.Now()) {
			return nil, errors.New("due date cannot be in the past")
		}
		existing.DueDate = updates.DueDate
	}

	if !updates.ReminderAt.IsZero() {
		if updates.ReminderAt.Before(time.Now()) {
			return nil, errors.New("reminder time cannot be in the past")
		}
		if !existing.DueDate.IsZero() && updates.ReminderAt.After(existing.DueDate) {
			return nil, errors.New("reminder time cannot be after due date")
		}
		existing.ReminderAt = updates.ReminderAt
	}

	// Update in repository
	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// Update Due Date
func (svc *TodosService) UpdateDueDate(ctx context.Context, todoID string, userID string, newDueDate time.Time) (*model.Todos, error) {
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	if !newDueDate.IsZero() && newDueDate.Before(time.Now()) {
		return nil, errors.New("due date cannot be in the past")
	}

	if !existing.ReminderAt.IsZero() && existing.ReminderAt.After(newDueDate) {
		return nil, errors.New("reminder time cannot be after due date")
	}

	existing.DueDate = newDueDate
	existing.UpdatedAt = time.Now()

	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// Update Reminder
func (svc *TodosService) UpdateReminder(ctx context.Context, todoID string, userID string, newReminder time.Time) (*model.Todos, error) {
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	if !newReminder.IsZero() {
		if newReminder.Before(time.Now()) {
			return nil, errors.New("reminder time cannot be in the past")
		}
		if !existing.DueDate.IsZero() && newReminder.After(existing.DueDate) {
			return nil, errors.New("reminder time cannot be after due date")
		}
	}

	existing.ReminderAt = newReminder
	existing.UpdatedAt = time.Now()

	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// Update Priority
func (svc *TodosService) UpdatePriority(ctx context.Context, todoID string, userID string, newPriority model.Priority) (*model.Todos, error) {
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	if err := validatePriority(newPriority); err != nil {
		return nil, err
	}

	existing.Priority = newPriority
	existing.UpdatedAt = time.Now()

	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// Update Recurrence
func (svc *TodosService) UpdateToRecurring(ctx context.Context, todoID string, userID string, pattern model.RecurrencePattern, endDate time.Time) (*model.Todos, error) {
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	if existing.DueDate.IsZero() {
		return nil, errors.New("cannot make todo recurring without due date")
	}

	// Validate recurrence pattern
	switch pattern {
	case model.RecurrenceDaily, model.RecurrenceWeekly, model.RecurrenceMonthly, model.RecurrenceYearly:
	default:
		return nil, errors.New("invalid recurrence pattern")
	}

	existing.IsRecurring = true
	existing.RecurrencePattern = pattern
	existing.RecurrenceEndDate = endDate
	existing.UpdatedAt = time.Now()

	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
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
	if days <= 0 {
		return nil, errors.New("days must be positive")
	}

	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	deadline := now.AddDate(0, 0, days)

	var upcomingTodos []*model.Todos
	for _, todo := range todos {
		if !todo.Complete && !todo.DueDate.IsZero() && todo.DueDate.After(now) && todo.DueDate.Before(deadline) {
			upcomingTodos = append(upcomingTodos, todo)
		}
	}

	return upcomingTodos, nil
}

// Updat Tags in Todos
func (svc *TodosService) UpdateTags(ctx context.Context, todoID string, userID string, newTags []string) (*model.Todos, error) {
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	validatedTags, err := svc.validateTags(newTags)
	if err != nil {
		return nil, err
	}

	existing.Tags = validatedTags
	existing.UpdatedAt = time.Now()

	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
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
func (svc *TodosService) ToggleTodoComplete(ctx context.Context, todoID string, userID string) (*model.Todos, error) {
	// Get todo
	existing, err := svc.repo.GetTodosByID(userID, todoID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("todo not found")
	}

	// Toggle complete status
	existing.Complete = !existing.Complete
	existing.UpdatedAt = time.Now()

	// Update in repository
	if err := svc.repo.UpdateTodo(ctx, todoID, userID, existing); err != nil {
		return nil, err
	}

	return existing, nil
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

// Todo Stats
func (svc *TodosService) GetTodoStats(ctx context.Context, userID string) (*model.TodoStats, error) {
	todos, err := svc.repo.GetUserTodos(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats := &model.TodoStats{
		Total:          len(todos),
		Completed:      0,
		Pending:        0,
		HighPriority:   0,
		MediumPriority: 0,
		LowPriority:    0,
		Overdue:        0,
		DueToday:       0,
		WithReminders:  0,
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	for _, todo := range todos {
		if todo.Complete {
			stats.Completed++
		} else {
			stats.Pending++
		}

		switch todo.Priority {
		case model.PriorityHigh:
			stats.HighPriority++
		case model.PriorityMedium:
			stats.MediumPriority++
		case model.PriorityLow:
			stats.LowPriority++
		}

		if !todo.Complete && !todo.DueDate.IsZero() {
			if todo.DueDate.Before(now) {
				stats.Overdue++
			} else if todo.DueDate.Before(today) {
				stats.DueToday++
			}
		}

		if !todo.ReminderAt.IsZero() {
			stats.WithReminders++
		}
	}

	return stats, nil
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

func getPriorityWeight(p model.Priority) int {
	switch p {
	case model.PriorityHigh:
		return 3
	case model.PriorityMedium:
		return 2
	case model.PriorityLow:
		return 1
	default:
		return 0
	}
}
