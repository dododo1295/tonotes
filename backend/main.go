package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"main/handler"
	"main/middleware"
	"main/repository"
	"main/services"
	"main/usecase"
	"main/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	shutdownTimeout = 30 * time.Second
	startupTimeout  = 30 * time.Second
)

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil && os.Getenv("GO_ENV") != "test" {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Required environment variables
	requiredEnvVars := []string{
		"MONGO_URI",
		"MONGO_DB",
		"USERS_COLLECTION",
		"JWT_SECRET_KEY",
		"JWT_EXPIRATION_TIME",
		"REFRESH_TOKEN_EXPIRATION_TIME",
		"SESSIONS_COLLECTION",
		"SESSION_DURATION",
		"REDIS_URL",
		"PORT",
		"MONGO_MAX_POOL_SIZE",
		"MONGO_MIN_POOL_SIZE",
		"MONGO_MAX_CONN_IDLE_TIME",
	}

	// Verify and log environment variables
	verifyEnvironment(requiredEnvVars)

	// Initialize services
	initializeServices()
}

func verifyEnvironment(requiredVars []string) {
	log.Println("Verifying environment variables:")
	missingVars := []string{}

	for _, envVar := range requiredVars {
		value := os.Getenv(envVar)
		if value == "" && os.Getenv("GO_ENV") != "test" {
			missingVars = append(missingVars, envVar)
			log.Printf("❌ %s: not set", envVar)
		} else {
			log.Printf("✅ %s: set", envVar)
		}
	}

	if len(missingVars) > 0 {
		log.Fatalf("Missing required environment variables: %v", missingVars)
	}
}

func collectSystemMetrics() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Update system metrics
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			utils.SystemMemoryUsage.Set(float64(m.Alloc))
			utils.SystemCPUUsage.Set(utils.GetCPUUsage())
			utils.GoroutineCount.Set(float64(runtime.NumGoroutine()))
		}
	}()
}

func initializeServices() {
	// Initialize services with proper error handling
	var initErrors []error
	var wg sync.WaitGroup
	errChan := make(chan error, 3) // Buffer for concurrent initialization errors

	// Initialize MongoDB with connection pooling
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := utils.InitMongoClient(); err != nil {
			errChan <- fmt.Errorf("MongoDB initialization error: %w", err)
		}
	}()

	// Initialize Redis services
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		// Initialize token blacklist
		wg.Add(1)
		go func() {
			defer wg.Done()
			blacklist, err := services.NewTokenBlacklist(redisURL)
			if err != nil {
				errChan <- fmt.Errorf("token blacklist initialization error: %w", err)
				return
			}
			services.TokenBlacklist = blacklist
		}()

		// Initialize session cache
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessionCache, err := services.NewSessionCache(redisURL)
			if err != nil {
				errChan <- fmt.Errorf("session cache initialization error: %w", err)
				return
			}
			services.GlobalSessionCache = sessionCache
		}()
	}

	// Initialize other services
	utils.InitValidator()
	utils.InitJWT()

	// Wait for all initializations and collect errors
	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		initErrors = append(initErrors, err)
	}

	if len(initErrors) > 0 {
		for _, err := range initErrors {
			log.Printf("Initialization error: %v", err)
		}
		log.Fatal("Failed to initialize one or more services")
	}
}

func setupRouter() *gin.Engine {
	// Create router with enhanced middleware
	router := gin.New()
	router.Use(
		gin.Logger(),
		middleware.EnhancedRecoveryMiddleware(),
		middleware.CORSMiddleware(),
		middleware.RequestTracingMiddleware(),
		utils.MetricsUtil(),
		middleware.RequestSizeLimiter(10<<20),
	)
	// initialize repository
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)
	notesRepo := repository.GetNotesRepo(utils.MongoClient)
	userRepo := repository.GetUserRepo(utils.MongoClient)
	todosService := usecase.NewTodosService(repository.GetTodosRepo(utils.MongoClient))

	// initialize services
	notesService := &usecase.NotesService{NotesRepo: notesRepo}

	// Initialize stats handler
	statsHandler := handler.NewStatsHandler(
		userRepo,
		notesRepo,
		todosService,
		sessionRepo,
	)

	// Metrics route
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check with detailed status
	router.GET("/health", func(c *gin.Context) {
		health := map[string]interface{}{
			"status":   "up",
			"time":     time.Now(),
			"services": make(map[string]string),
		}

		// Check MongoDB connection
		if err := utils.CheckMongoConnection(); err != nil {
			health["services"].(map[string]string)["mongodb"] = "down"
			utils.TrackDependencyHealth("mongodb", "connection", false)
		} else {
			health["services"].(map[string]string)["mongodb"] = "up"
			utils.TrackDependencyHealth("mongodb", "connection", true)
		}

		// Track API health
		utils.TrackAPIHealth("/health", true)

		c.JSON(http.StatusOK, health)
	})

	// Set up routes
	setupRoutes(router, sessionRepo, notesService, statsHandler, todosService)

	return router
}

func setupRoutes(
	router *gin.Engine,
	sessionRepo *repository.SessionRepo,
	notesService *usecase.NotesService,
	statsHandler *handler.StatsHandler,
	todosService *usecase.TodosService,
) {
	// Initialize handlers
	notesHandler := handler.NewNotesHandler(notesService)
	todosHandler := handler.NewTodosHandler(todosService)

	// Public routes
	auth := router.Group("")
	{
		auth.POST("/register", handler.RegistrationHandler)
		auth.POST("/login", func(c *gin.Context) {
			handler.LoginHandler(c, sessionRepo)
		})
		auth.POST("/logout", func(c *gin.Context) {
			handler.LogoutHandler(c, sessionRepo)
		})
		auth.POST("/token/refresh", handler.RefreshTokenHandler)
	}

	// Protected routes
	protected := router.Group("")
	protected.Use(
		middleware.AuthMiddleware(),
		middleware.SessionMiddleware(sessionRepo),
	)

	// User routes
	user := protected.Group("/user")
	{
		user.PUT("/email", handler.ChangeEmailHandler)
		user.PUT("/password", handler.ChangePasswordHandler)
		user.DELETE("", handler.DeleteUserHandler)
		user.GET("/profile", handler.GetUserProfileHandler)
	}

	// Session routes
	sessions := protected.Group("/sessions")
	{
		sessions.GET("", func(c *gin.Context) {
			handler.GetActiveSessions(c, sessionRepo)
		})
		sessions.POST("/logout-all", func(c *gin.Context) {
			handler.LogoutAllSessions(c, sessionRepo)
		})
		sessions.POST("/:session_id/logout", func(c *gin.Context) {
			handler.LogoutSession(c, sessionRepo)
		})
		sessions.GET("/:session_id", func(c *gin.Context) {
			handler.GetSessionDetails(c, sessionRepo)
		})
	}

	// Stats routes
	stats := protected.Group("/stats")
	{
		stats.GET("", statsHandler.GetUserStats)
	}

	// 2FA routes
	twoFA := protected.Group("/2fa")
	{
		twoFA.POST("/enable", handler.Enable2FAHandler)
		twoFA.POST("/verify", handler.Verify2FAHandler)
		twoFA.POST("/disable", handler.Disable2FAHandler)
		twoFA.POST("/recovery", handler.UseRecoveryCodeHandler)
		twoFA.GET("/setup", handler.Generate2FASecretHandler)
	}

	// Notes routes
	notes := protected.Group("/notes")
	{
		// Basic CRUD
		notes.GET("", notesHandler.SearchNotes)
		notes.POST("", notesHandler.CreateNote)
		notes.PUT("/:id", notesHandler.UpdateNote)
		notes.DELETE("/:id", notesHandler.DeleteNote)

		// Note actions
		notes.POST("/:id/favorite", notesHandler.ToggleFavorite)
		notes.POST("/:id/pin", notesHandler.TogglePin)
		notes.POST("/:id/archive", notesHandler.ArchiveNote)
		notes.PUT("/:id/pin-position", notesHandler.UpdatePinPosition)

		// Note queries
		notes.GET("/tags", notesHandler.GetUserTags)
		notes.GET("/suggestions", notesHandler.GetSearchSuggestions)
		notes.GET("/archived", notesHandler.GetArchivedNotes)
	}

	// Todos routes
	todos := protected.Group("/todos")
	{
		// Create and List
		todos.POST("", todosHandler.CreateTodo)
		todos.GET("", todosHandler.GetUserTodos)

		// Basic CRUD operations
		todos.PUT("/:id", todosHandler.UpdateTodo)
		todos.DELETE("/:id", todosHandler.DeleteTodo)

		// Status updates
		todos.POST("/:id/complete", todosHandler.ToggleTodoComplete)

		// Field updates
		todos.PUT("/:id/due-date", todosHandler.UpdateDueDate)
		todos.PUT("/:id/reminder", todosHandler.UpdateReminder)
		todos.PUT("/:id/priority", todosHandler.UpdatePriority)
		todos.PUT("/:id/tags", todosHandler.UpdateTags)
		todos.PUT("/:id/recurring", todosHandler.UpdateToRecurring)

		// Filtered views
		todos.GET("/search", todosHandler.SearchTodos)
		todos.GET("/priority", todosHandler.GetTodosByPriority)
		todos.GET("/tags", todosHandler.GetTodosByTags)
		todos.GET("/upcoming", todosHandler.GetUpcomingTodos)
		todos.GET("/overdue", todosHandler.GetOverdueTodos)
		todos.GET("/completed", todosHandler.GetCompletedTodos)
		todos.GET("/pending", todosHandler.GetPendingTodos)
		todos.GET("/with-reminders", todosHandler.GetTodosWithReminders)

		// Metadata
		todos.GET("/user-tags", todosHandler.GetUserTags)
		todos.GET("/stats", todosHandler.GetTodoStats)
		todos.GET("/count", todosHandler.CountUserTodos)
	}
}

func main() {
	// Initialize validator
	validate := validator.New()
	validate.RegisterValidation("password", utils.ValidatePasswordRule)

	// Set up router and server
	router := setupRouter()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	collectSystemMetrics()

	// Start server
	go func() {
		log.Printf("Server starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start MongoDB health check routine
	go monitorDatabaseHealth()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	performGracefulShutdown(srv)
}

func monitorDatabaseHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := utils.CheckMongoConnection(); err != nil {
			log.Printf("MongoDB health check failed: %v", err)
		}
	}
}

func performGracefulShutdown(srv *http.Server) {
	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Clean up resources
	cleanup(ctx)
}

func cleanup(ctx context.Context) {
	// Record start time for MTTR calculation
	startTime := time.Now()

	// Create WaitGroup for cleanup tasks
	var wg sync.WaitGroup
	errChan := make(chan error, 3) // Buffer for cleanup errors
	done := make(chan struct{})

	// Cleanup MongoDB
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := utils.CloseMongoConnection(); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
			errChan <- err
			utils.TrackDependencyHealth("mongodb", "cleanup", false)
			utils.TrackError("cleanup", "mongodb_disconnect")
		} else {
			utils.TrackDependencyHealth("mongodb", "cleanup", true)
		}
	}()

	// Cleanup Redis Token Blacklist
	if services.TokenBlacklist != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := services.TokenBlacklist.Close(); err != nil {
				log.Printf("Error closing token blacklist: %v", err)
				errChan <- err
				utils.TrackDependencyHealth("redis", "blacklist_cleanup", false)
				utils.TrackError("cleanup", "redis_blacklist")
			} else {
				utils.TrackDependencyHealth("redis", "blacklist_cleanup", true)
			}
		}()
	}

	// Cleanup Redis Session Cache
	if services.GlobalSessionCache != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := services.GlobalSessionCache.Close(); err != nil {
				log.Printf("Error closing session cache: %v", err)
				errChan <- err
				utils.TrackDependencyHealth("redis", "session_cache_cleanup", false)
				utils.TrackError("cleanup", "redis_session_cache")
			} else {
				utils.TrackDependencyHealth("redis", "session_cache_cleanup", true)
			}
		}()
	}

	// Wait for all cleanup tasks and collect errors
	var cleanupErrors []error
	go func() {
		wg.Wait()
		close(errChan)
		close(done)
	}()

	// Handle cleanup completion or timeout
	select {
	case <-done:
		// Collect any errors that occurred during cleanup
		for err := range errChan {
			if err != nil {
				cleanupErrors = append(cleanupErrors, err)
			}
		}

		if len(cleanupErrors) == 0 {
			log.Println("Cleanup completed successfully")
			utils.UpdateMTTR(time.Since(startTime).Minutes())
			utils.TrackDependencyHealth("application", "shutdown", true)
		} else {
			log.Printf("Cleanup completed with %d errors", len(cleanupErrors))
			for _, err := range cleanupErrors {
				log.Printf("Cleanup error: %v", err)
				utils.TrackError("cleanup", "failed")
			}
			utils.UpdateMTTR(time.Since(startTime).Minutes())
			utils.TrackDependencyHealth("application", "shutdown", false)
		}

	case <-ctx.Done():
		log.Println("Cleanup timed out")
		utils.TrackError("cleanup", "timeout")
		utils.UpdateMTTR(time.Since(startTime).Minutes())
		utils.TrackDependencyHealth("application", "shutdown", false)
	}
}
