package handler

import (
	"main/dto"
	"main/model"
	"main/usecase"
	"main/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TodoHandler struct {
	service *usecase.TodoService
}

func NewTodoHandler(service *usecase.TodoService) *TodoHandler {
	return &TodoHandler{service: service}
}

func (h *TodoHandler) CreateTodo(c *gin.Context) {
	// Get authenticated user ID
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	// Define request structure matching model fields
	var req struct {
		TodoName          string                  `json:"todo_name" binding:"required"`
		Description       string                  `json:"description"`
		Priority          model.Priority          `json:"priority"`
		Tags              []string                `json:"tags"`
		DueDate           time.Time               `json:"due_date"`
		ReminderAt        time.Time               `json:"reminder_at"`
		IsRecurring       bool                    `json:"is_recurring"`
		RecurrencePattern model.RecurrencePattern `json:"recurrence_pattern,omitempty"`
		RecurrenceEndDate time.Time               `json:"recurrence_end_date,omitempty"`
	}

	// Bind and validate request body
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// Initialize new todo with request data
	todo := &model.Todo{
		UserID:            userID.(string),
		TodoName:          req.TodoName,
		Description:       req.Description,
		Priority:          req.Priority,
		Tags:              req.Tags,
		DueDate:           req.DueDate,
		ReminderAt:        req.ReminderAt,
		IsRecurring:       req.IsRecurring,
		RecurrencePattern: req.RecurrencePattern,
		RecurrenceEndDate: req.RecurrenceEndDate,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Validate required fields
	if todo.TodoName == "" {
		utils.BadRequest(c, "Todo name is required")
		return
	}

	// Validate due date if provided
	if !todo.DueDate.IsZero() && todo.DueDate.Before(time.Now()) {
		utils.BadRequest(c, "Due date cannot be in the past")
		return
	}

	// Validate reminder time if provided
	if !todo.ReminderAt.IsZero() {
		if todo.ReminderAt.Before(time.Now()) {
			utils.BadRequest(c, "Reminder time cannot be in the past")
			return
		}
		if !todo.DueDate.IsZero() && todo.ReminderAt.After(todo.DueDate) {
			utils.BadRequest(c, "Reminder time cannot be after due date")
			return
		}
	}

	// Validate recurrence settings for recurring todos
	if todo.IsRecurring {
		if todo.DueDate.IsZero() {
			utils.BadRequest(c, "Due date is required for recurring todos")
			return
		}
		// Validate recurrence pattern
		switch todo.RecurrencePattern {
		case model.RecurrenceDaily, model.RecurrenceWeekly, model.RecurrenceMonthly, model.RecurrenceYearly:
		default:
			utils.BadRequest(c, "Invalid recurrence pattern")
			return
		}
	}

	// Delegate todo creation to service layer
	if err := h.service.CreateTodo(c.Request.Context(), todo); err != nil {
		// Handle validation errors
		if strings.Contains(err.Error(), "invalid priority level") ||
			strings.Contains(err.Error(), "cannot exceed 5 tags") ||
			strings.Contains(err.Error(), "tag cannot exceed 20 characters") {
			utils.BadRequest(c, err.Error())
			return
		}
		// Handle internal errors
		utils.InternalError(c, err.Error())
		return
	}

	// Convert to response object and return
	response := dto.ToTodoResponse(todo)
	utils.Created(c, response)
}

func (h *TodoHandler) GetUserTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todos, err := h.service.GetUserTodos(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) UpdateTodo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var updates model.Todo
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequest(c, "Invalid request body")
		return
	}

	updatedTodo, err := h.service.UpdateTodo(c.Request.Context(), todoID, userID.(string), &updates)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) DeleteTodo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	if err := h.service.DeleteTodo(c.Request.Context(), todoID, userID.(string)); err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "Todo deleted successfully"})
}

func (h *TodoHandler) SearchTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	searchText := c.Query("q")
	todos, err := h.service.SearchTodos(c.Request.Context(), userID.(string), searchText)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetTodosByPriority(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	priority := model.Priority(c.Query("priority"))
	todos, err := h.service.GetTodosByPriority(c.Request.Context(), userID.(string), priority)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetTodosByTags(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	tags := c.QueryArray("tags")
	todos, err := h.service.GetTodosByTags(c.Request.Context(), userID.(string), tags)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetUserTags(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	tags, err := h.service.GetUserTags(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, tags)
}

func (h *TodoHandler) GetUpcomingTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		utils.BadRequest(c, "Invalid days parameter, must be positive")
		return
	}

	todos, err := h.service.GetUpcomingTodos(c.Request.Context(), userID.(string), days)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetOverdueTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todos, err := h.service.GetOverdueTodos(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) ToggleTodoComplete(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	updatedTodo, err := h.service.ToggleTodoComplete(c.Request.Context(), todoID, userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) UpdateDueDate(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var req struct {
		DueDate time.Time `json:"due_date"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid due date")
		return
	}

	updatedTodo, err := h.service.UpdateDueDate(c.Request.Context(), todoID, userID.(string), req.DueDate)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) UpdateReminder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var req struct {
		ReminderAt time.Time `json:"reminder_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid reminder time")
		return
	}

	updatedTodo, err := h.service.UpdateReminder(c.Request.Context(), todoID, userID.(string), req.ReminderAt)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) UpdatePriority(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var req struct {
		Priority model.Priority `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid priority")
		return
	}

	updatedTodo, err := h.service.UpdatePriority(c.Request.Context(), todoID, userID.(string), req.Priority)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) GetCompletedTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todos, err := h.service.GetCompletedTodos(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetPendingTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todos, err := h.service.GetPendingTodos(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) GetTodoStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	stats, err := h.service.GetTodoStats(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, stats)
}

func (h *TodoHandler) GetTodosWithReminders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todos, err := h.service.GetTodosWithReminders(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	responses := dto.ToTodoResponses(todos)
	utils.Success(c, responses)
}

func (h *TodoHandler) UpdateTags(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid tags")
		return
	}

	updatedTodo, err := h.service.UpdateTags(c.Request.Context(), todoID, userID.(string), req.Tags)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}
func (h *TodoHandler) UpdateToRecurring(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	todoID := c.Param("id")
	if todoID == "" {
		utils.BadRequest(c, "Missing todo ID")
		return
	}

	var req struct {
		Pattern model.RecurrencePattern `json:"pattern"`
		EndDate time.Time               `json:"end_date"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid recurrence data")
		return
	}

	updatedTodo, err := h.service.UpdateToRecurring(c.Request.Context(), todoID, userID.(string), req.Pattern, req.EndDate)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	response := dto.ToTodoResponse(updatedTodo)
	utils.Success(c, response)
}

func (h *TodoHandler) CountUserTodos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing user ID")
		return
	}

	count, err := h.service.CountUserTodos(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, gin.H{"count": count})
}
