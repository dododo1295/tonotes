package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
		middleware.MetricsMiddleware(),
		middleware.RequestSizeLimiter(10<<20),
	)
	// initialize repository
	sessionRepo := repository.GetSessionRepo(utils.MongoClient)
	notesRepo := repository.GetNotesRepo(utils.MongoClient)
	userRepo := repository.GetUserRepo(utils.MongoClient)
	todosRepo := repository.GetTodosRepo(utils.MongoClient)

	// initialize services
	notesService := &usecase.NotesService{NotesRepo: notesRepo}

	// Initialize stats handler
	statsHandler := handler.NewStatsHandler(
		userRepo,
		notesRepo,
		todosRepo,
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
		} else {
			health["services"].(map[string]string)["mongodb"] = "up"
		}

		c.JSON(http.StatusOK, health)
	})

	// Set up routes
	setupRoutes(router, sessionRepo, notesService, statsHandler)

	return router
}

func setupRoutes(router *gin.Engine, sessionRepo *repository.SessionRepo, notesService *usecase.NotesService, statsHandler *handler.StatsHandler) {

	// Auth routes
	router.POST("/register", handler.RegistrationHandler)
	router.POST("/login", func(c *gin.Context) {
		handler.LoginHandler(c, sessionRepo)
	})
	router.POST("/logout", func(c *gin.Context) {
		handler.LogoutHandler(c, sessionRepo)
	})
	router.POST("/token/refresh", handler.RefreshTokenHandler)

	// Protected routes group
	protected := router.Group("")
	protected.Use(
		middleware.AuthMiddleware(),
		func(c *gin.Context) {
			middleware.SessionMiddleware(sessionRepo)(c)
		},
	)

	// User management
	protected.PUT("/user/email", handler.ChangeEmailHandler)
	protected.PUT("/user/password", handler.ChangePasswordHandler)
	protected.DELETE("/user", handler.DeleteUserHandler)
	protected.GET("/user/profile", handler.GetUserProfileHandler)

	// Session management
	protected.GET("/sessions", func(c *gin.Context) {
		handler.GetActiveSessions(c, sessionRepo)
	})
	protected.POST("/sessions/logout-all", func(c *gin.Context) {
		handler.LogoutAllSessions(c, sessionRepo)
	})
	protected.POST("/sessions/:session_id/logout", func(c *gin.Context) {
		handler.LogoutSession(c, sessionRepo)
	})
	protected.GET("/sessions/:session_id", func(c *gin.Context) {
		handler.GetSessionDetails(c, sessionRepo)
	})

	// Stats routes
	protected.GET("/stats", statsHandler.GetUserStats)

	// 2FA routes
	protected.POST("/2fa/enable", handler.Enable2FAHandler)
	protected.POST("/2fa/verify", handler.Verify2FAHandler)
	protected.POST("/2fa/disable", handler.Disable2FAHandler)
	protected.POST("/2fa/recovery", handler.UseRecoveryCodeHandler)
	protected.GET("/2fa/setup", handler.Generate2FASecretHandler)

	// Notes routes
	protected.GET("/notes", func(c *gin.Context) {
		handler.SearchNotesHandler(c, notesService)
	})
	protected.POST("/notes", func(c *gin.Context) {
		handler.CreateNoteHandler(c, notesService)
	})
	protected.PUT("/notes/:id", func(c *gin.Context) {
		handler.UpdateNoteHandler(c, notesService)
	})
	protected.DELETE("/notes/:id", func(c *gin.Context) {
		handler.DeleteNoteHandler(c, notesService)
	})
	protected.POST("/notes/:id/favorite", func(c *gin.Context) {
		handler.ToggleFavoriteHandler(c, notesService)
	})
	protected.POST("/notes/:id/pin", func(c *gin.Context) {
		handler.TogglePinHandler(c, notesService)
	})
	protected.POST("/notes/:id/archive", func(c *gin.Context) {
		handler.ArchiveNoteHandler(c, notesService)
	})
	protected.GET("/notes/tags", func(c *gin.Context) {
		handler.GetUserTagsHandler(c, notesService)
	})
	protected.GET("/notes/suggestions", func(c *gin.Context) {
		handler.GetSearchSuggestionsHandler(c, notesService)
	})
	protected.GET("/notes/archived", func(c *gin.Context) {
		handler.GetArchivedNotesHandler(c, notesService)
	})
	protected.PUT("/notes/:id/pin-position", func(c *gin.Context) {
		handler.UpdatePinPositionHandler(c, notesService)
	})
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
	// Create WaitGroup for cleanup tasks
	var wg sync.WaitGroup

	// Cleanup MongoDB
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := utils.CloseMongoConnection(); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Cleanup Redis services
	if services.TokenBlacklist != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := services.TokenBlacklist.Close(); err != nil {
				log.Printf("Error closing token blacklist: %v", err)
			}
		}()
	}

	if services.GlobalSessionCache != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := services.GlobalSessionCache.Close(); err != nil {
				log.Printf("Error closing session cache: %v", err)
			}
		}()
	}

	// Wait for all cleanup tasks with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Cleanup completed successfully")
	case <-ctx.Done():
		log.Println("Cleanup timed out")
	}
}
