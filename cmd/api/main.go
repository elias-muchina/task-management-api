package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-manager-api/internal/config"
	"task-manager-api/internal/handlers"
	"task-manager-api/internal/middleware"
	"task-manager-api/internal/repository"
	"task-manager-api/internal/service"
	"task-manager-api/internal/utils"
	"task-manager-api/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Set Gin mode
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize PostgreSQL
	pgPool, err := database.NewPostgresPool(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgPool.Close()

	// Get a connection from the pool
	ctx := context.Background()
	conn, err := pgPool.Acquire(ctx)
	if err != nil {
		log.Fatalf("Failed to acquire connection: %v", err)
	}
	defer conn.Release()

	// Initialize Redis (optional)
	var redisClient *redis.Client
	if cfg.Redis.Host != "" && cfg.Redis.Host != "disabled" {
		redisClient, err = database.NewRedisClient(&cfg.Redis)
		if err != nil {
			log.Printf("Warning: Redis connection failed: %v", err)
			log.Println("Continuing without Redis...")
			redisClient = nil
		} else {
			defer redisClient.Close()
		}
	}

	// Initialize JWT
	utils.InitJWT(cfg.JWT.Secret)

	// Initialize repositories
	userRepo := repository.NewUserRepository(conn.Conn())
	taskRepo := repository.NewTaskRepository(conn.Conn(), redisClient)

	// Initialize services
	taskService := service.NewTaskService(taskRepo)
	taskWorker := service.NewTaskWorker(10, taskRepo)

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskService, taskWorker)
	authHandler := handlers.NewAuthHandler(userRepo)

	// Setup router
	router := gin.Default()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Rate limiting middleware (skip if Redis is nil)
	if redisClient != nil {
		router.Use(middleware.RateLimitMiddleware(
			redisClient,
			cfg.RateLimit.Requests,
			cfg.RateLimit.Window,
		))
	} else {
		log.Println("Rate limiting disabled (Redis not available)")
	}

	// Public routes
	router.GET("/health", handlers.HealthCheck)
	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)

	// Protected routes
	authGroup := router.Group("/api")
	authGroup.Use(middleware.AuthMiddleware())
	{
		authGroup.GET("/tasks", taskHandler.GetTasks)
		authGroup.POST("/tasks", taskHandler.CreateTask)
		authGroup.GET("/tasks/:id", taskHandler.GetTask)
		authGroup.PUT("/tasks/:id", taskHandler.UpdateTask)
		authGroup.DELETE("/tasks/:id", taskHandler.DeleteTask)
		authGroup.POST("/tasks/batch", taskHandler.BatchProcessTasks)
	}

	// Start server with graceful shutdown
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
