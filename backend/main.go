package main

import (
	"fmt"
	"log"
	"os"

	"main/handler"
	"main/middleware"
	"main/repository"
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
		"SESSION_COLLECTION", // Added for sessions
		"SESSION_DURATION",   // Added for sessions
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
	utils.InitValidator()
	// Initialize JWT
	utils.InitJWT()
	// Initialize MongoDB connection
	utils.InitMongoClient()
}

func setupRouter() *gin.Engine {
	// Create default gin router
	router := gin.Default()

	// Initialize session repository
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)
	notesRepo := repository.GetNotesRepo(utils.MongoClient)

	// Initialize services
	notesService := &usecase.NotesService{
		NotesRepo: notesRepo,
	}

	// Add CORS middleware
	router.Use(middleware.CORSMiddleware())

	// Add session middleware
	router.Use(middleware.SessionMiddleware(sessionRepo))

	// Public routes (no authentication required)
	public := router.Group("/api")
	{
		auth := public.Group("/auth")
		{
			auth.POST("/register", handler.RegistrationHandler)
			// Inject session repo into login handler
			auth.POST("/login", func(c *gin.Context) {
				handler.LoginHandler(c, sessionRepo)
			})
		}
	}

	// Protected routes (authentication required)
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		// User management
		user := protected.Group("/user")
		{
			user.GET("/profile", handler.GetUserProfileHandler)
			user.POST("/change-email", handler.ChangeEmailHandler)
			user.POST("/change-password", handler.ChangePasswordHandler)
			// Inject session repo into logout handler
			user.POST("/logout", func(c *gin.Context) {
				handler.LogoutHandler(c, sessionRepo)
			})
			user.DELETE("/delete", handler.DeleteUserHandler)
		}

		// Session management endpoints
		sessions := protected.Group("/sessions")
		{
			sessions.GET("/active", func(c *gin.Context) {
				handler.GetActiveSessions(c, sessionRepo)
			})
			sessions.POST("/logout-all", func(c *gin.Context) {
				handler.LogoutAllSessions(c, sessionRepo)
			})
		}

		// Notes endpoints (to be implemented)
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

		// Todos endpoints (to be implemented)
		todos := protected.Group("/todos")
		{
			todos.GET("/", nil)       // List todos
			todos.POST("/", nil)      // Create todo
			todos.PUT("/:id", nil)    // Update todo
			todos.DELETE("/:id", nil) // Delete todo
		}
	}

	return router
}

func main() {
	validate := validator.New()
	validate.RegisterValidation("password", utils.ValidatePasswordRule)

	// Set up router
	router := setupRouter()

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	serverAddr := fmt.Sprintf(":%s", port)
	log.Printf("Server starting on %s", serverAddr)
	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
