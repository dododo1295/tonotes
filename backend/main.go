package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
)

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil && os.Getenv("GO_ENV") != "test" {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Check required environment variables
	requiredEnvVars := []string{
		"MONGO_URI",
		"MONGO_DB",
		"USERS_COLLECTION",
		"JWT_SECRET_KEY",
		"JWT_EXPIRATION_TIME",
		"REFRESH_TOKEN_EXPIRATION_TIME",
		"SESSION_COLLECTION",
		"SESSION_DURATION",
		"REDIS_URL",
		"PORT",
	}

	// Print environment variables for debugging
	log.Println("Environment variables:")
	for _, envVar := range requiredEnvVars {
		value := os.Getenv(envVar)
		if value == "" {
			log.Printf("%s: not set", envVar)
		} else {
			log.Printf("%s: set", envVar)
		}
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" && os.Getenv("GO_ENV") != "test" {
			log.Fatalf("Required environment variable %s is not set", envVar)
		}
	}

	// Initialize Redis services
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL environment variable is not set")
	}

	// Initialize token blacklist
	blacklist, err := services.NewTokenBlacklist(redisURL)
	if err != nil {
		log.Fatalf("Failed to initialize token blacklist: %v", err)
	}
	services.TokenBlacklist = blacklist

	// Initialize session cache
	sessionCache, err := services.NewSessionCache(redisURL)
	if err != nil {
		log.Fatalf("Failed to initialize session cache: %v", err)
	}
	services.GlobalSessionCache = sessionCache

	// Initialize other services
	utils.InitValidator()
	utils.InitJWT()
	utils.InitMongoClient()
}

func setupRouter() *gin.Engine {
	// Create router with recovery and logging middleware
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Add health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "up",
			"time":   time.Now(),
		})
	})

	// Initialize repositories
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)
	notesRepo := repository.GetNotesRepo(utils.MongoClient)

	// Initialize services
	notesService := &usecase.NotesService{
		NotesRepo: notesRepo,
	}

	// Add middleware
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SessionMiddleware(sessionRepo))

	// Public routes
	public := router.Group("/api")
	{
		auth := public.Group("/auth")
		{
			auth.POST("/register", handler.RegistrationHandler)
			auth.POST("/login", func(c *gin.Context) {
				handler.LoginHandler(c, sessionRepo)
			})
			auth.POST("/refresh", handler.RefreshTokenHandler)
		}
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		// User management
		user := protected.Group("/user")
		{
			user.GET("/profile", handler.GetUserProfileHandler)
			user.POST("/change-email", handler.ChangeEmailHandler)
			user.POST("/change-password", handler.ChangePasswordHandler)
			user.POST("/logout", func(c *gin.Context) {
				handler.LogoutHandler(c, sessionRepo)
			})
			user.DELETE("/delete", handler.DeleteUserHandler)
		}

		// Session management
		sessions := protected.Group("/sessions")
		{
			// showing active sessions
			sessions.GET("/active", func(c *gin.Context) {
				handler.GetActiveSessions(c, sessionRepo)
			})
			// logout of all sessions
			sessions.POST("/logout-all", func(c *gin.Context) {
				handler.LogoutAllSessions(c, sessionRepo)
			})
			// get session details for listing
			sessions.GET("/:session_id", func(c *gin.Context) {
				handler.GetSessionDetails(c, sessionRepo)
			})

			// user force update session list
			sessions.PATCH("/:session_id/refresh", func(c *gin.Context) {
				handler.UpdateSession(c, sessionRepo)
			})
		}

		// Notes endpoints
		notes := protected.Group("/notes")
		{
			// Search and list operations
			notes.GET("/", func(c *gin.Context) {
				handler.GetUserNotesHandler(c, notesService)
			})
			notes.GET("/search", func(c *gin.Context) {
				handler.SearchNotesHandler(c, notesService)
			})
			notes.GET("/archived", func(c *gin.Context) {
				handler.GetArchivedNotesHandler(c, notesService)
			})
			notes.GET("/pinned", func(c *gin.Context) {
				handler.GetPinnedNotesHandler(c, notesService)
			})

			// Tag-related operations
			notes.GET("/tags", func(c *gin.Context) {
				handler.GetUserTagsHandler(c, notesService)
			})
			notes.GET("/tags/all", func(c *gin.Context) {
				handler.GetAllUserTagsHandler(c, notesService)
			})
			notes.GET("/suggestions", func(c *gin.Context) {
				handler.GetSearchSuggestionsHandler(c, notesService)
			})

			// Basic CRUD operations
			notes.POST("/", func(c *gin.Context) {
				handler.CreateNoteHandler(c, notesService)
			})
			notes.PUT("/:id", func(c *gin.Context) {
				handler.UpdateNoteHandler(c, notesService)
			})
			notes.DELETE("/:id", func(c *gin.Context) {
				handler.DeleteNoteHandler(c, notesService)
			})

			// Note actions
			notes.POST("/:id/favorite", func(c *gin.Context) {
				handler.ToggleFavoriteHandler(c, notesService)
			})
			notes.POST("/:id/pin", func(c *gin.Context) {
				handler.TogglePinHandler(c, notesService)
			})
			notes.POST("/:id/archive", func(c *gin.Context) {
				handler.ArchiveNoteHandler(c, notesService)
			})
			notes.PUT("/:id/position", func(c *gin.Context) {
				handler.UpdatePinPositionHandler(c, notesService)
			})
		}

		// Todos endpoints (placeholder)
		todos := protected.Group("/todos")
		{
			todos.GET("/", nil)
			todos.POST("/", nil)
			todos.PUT("/:id", nil)
			todos.DELETE("/:id", nil)
		}
	}

	return router
}

func main() {
	validate := validator.New()
	validate.RegisterValidation("password", utils.ValidatePasswordRule)

	// Set up router
	router := setupRouter()

	// Create server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Graceful shutdown handling
	go func() {
		log.Printf("Server starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Shutdown gracefully
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	// Clean up resources
	if err := utils.MongoClient.Disconnect(ctx); err != nil {
		log.Printf("Error disconnecting from MongoDB: %v", err)
	}

	if services.TokenBlacklist != nil {
		if err := services.TokenBlacklist.Close(); err != nil {
			log.Printf("Error closing token blacklist: %v", err)
		}
	}

	if services.GlobalSessionCache != nil {
		if err := services.GlobalSessionCache.Close(); err != nil {
			log.Printf("Error closing session cache: %v", err)
		}
	}

	log.Println("Server exited properly")
}
