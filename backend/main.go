package main

import (
	"fmt"
	"log"
	"os"

	"main/handler"
	"main/middleware"
	"main/repository"
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
		"SESSION_COLLECTION",  // Added for sessions
		"SESSION_DURATION",    // Added for sessions
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
			notes.GET("/", nil)       // List notes
			notes.POST("/", nil)      // Create note
			notes.PUT("/:id", nil)    // Update note
			notes.DELETE("/:id", nil) // Delete note
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
