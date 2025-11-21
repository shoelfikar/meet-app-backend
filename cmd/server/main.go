package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/meet-app/backend/internal/api/handlers"
	"github.com/meet-app/backend/internal/api/middleware"
	"github.com/meet-app/backend/internal/config"
	"github.com/meet-app/backend/internal/models"
	"github.com/meet-app/backend/internal/repository"
	"github.com/meet-app/backend/internal/service"
	"github.com/meet-app/backend/internal/sse"
	"github.com/meet-app/backend/pkg/database"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Set Gin mode
	gin.SetMode(cfg.Server.GinMode)

	// Initialize database connections
	if err := database.InitPostgres(&cfg.Database); err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer database.ClosePostgres()

	if err := database.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer database.CloseRedis()

	// Auto-migrate database schema
	db := database.GetDB()
	if err := db.AutoMigrate(
		&models.User{},
		&models.Meeting{},
		&models.Participant{},
		&models.Message{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("✅ Database migration completed")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	participantRepo := repository.NewParticipantRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, &cfg.JWT)
	meetingService := service.NewMeetingService(meetingRepo, participantRepo)
	messageService := service.NewMessageService(messageRepo, participantRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)
	meetingHandler := handlers.NewMeetingHandler(meetingService, messageService)
	sseHandler := sse.NewHandler()

	// Initialize router
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.LoggingMiddleware())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.ErrorHandlerMiddleware())
	router.Use(middleware.CORSMiddleware())

	// Health check endpoints
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Meet App Backend is running",
			"version": "1.0.0",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ready",
		})
	})

	// API routes
	api := router.Group("/api")
	{
		// Auth routes (public)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)

			// Protected auth routes
			authProtected := auth.Group("")
			authProtected.Use(middleware.AuthMiddleware(&cfg.JWT))
			{
				authProtected.GET("/me", authHandler.GetMe)
				authProtected.POST("/logout", authHandler.Logout)
			}
		}

		// Meeting routes (protected)
		meetings := api.Group("/meetings")
		meetings.Use(middleware.AuthMiddleware(&cfg.JWT))
		{
			meetings.POST("", meetingHandler.CreateMeeting)
			meetings.POST("/join", meetingHandler.JoinMeeting)
			meetings.GET("/code/:code", meetingHandler.GetMeetingByCode)

			// Meeting ID-based routes
			meetingByID := meetings.Group("/:id")
			{
				meetingByID.POST("/leave", meetingHandler.LeaveMeeting)
				meetingByID.POST("/end", meetingHandler.EndMeeting)
				meetingByID.GET("/participants", meetingHandler.GetMeetingParticipants)
				meetingByID.POST("/messages", meetingHandler.SendMessage)
				meetingByID.GET("/messages", meetingHandler.GetMessages)
				meetingByID.GET("/events", sseHandler.Stream)
			}
		}
	}

	// WebSocket endpoint (protected)
	router.GET("/ws", middleware.AuthMiddleware(&cfg.JWT), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "WebSocket endpoint - Not implemented yet"})
	})

	// Start server
	log.Printf("🚀 Server starting on port %s", cfg.Server.Port)
	log.Printf("📍 Environment: %s", cfg.Server.Environment)
	log.Printf("📍 Health check: http://localhost:%s/health", cfg.Server.Port)
	log.Printf("📍 API: http://localhost:%s/api", cfg.Server.Port)

	if err := router.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
