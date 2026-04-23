package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	authHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/auth"
	productHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/products"
	userHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/users"
	productRepo "github.com/ganiramadhan/ganipedia/backend/internal/repository/product"
	userRepo "github.com/ganiramadhan/ganipedia/backend/internal/repository/user"
	"github.com/ganiramadhan/ganipedia/backend/internal/routes"
	authSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/auth"
	cleanupSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/cleanup"
	productSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/product"
	userSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/user"

	_ "github.com/ganiramadhan/ganipedia/backend/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
)

// @title           Ganipedia API
// @version         1.0
// @description     Ganipedia Backend API - Product Management
// @termsOfService  http://swagger.io/terms/

// @contact.name   Gani Ramadhan
// @contact.email  gani@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:4000
// @BasePath  /

// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	// Load environment variables
	config.LoadEnv()

	// Connect to database
	config.ConnectDatabase()

	// Run database migrations
	config.RunMigrations()

	// Connect to Redis
	if err := config.ConnectRedis(); err != nil {
		log.Printf("Warning: %v - Redis caching will be disabled", err)
	}

	// Connect to S3/MinIO
	config.ConnectS3()

	// Ensure graceful shutdown
	defer config.CloseConnections()

	// Initialize Fiber with production-ready config
	app := fiber.New(fiber.Config{
		AppName:      "Ganipedia API v1.0",
		BodyLimit:    10 * 1024 * 1024, // 10MB
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${locals:requestid} | ${status} | ${latency} | ${ip} | ${method} ${path} | ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     config.GetEnv("CORS_ORIGINS", "*"),
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		AllowCredentials: false,
		MaxAge:           300, // 5 minutes
	}))

	// Swagger
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Health check with dependency status
	app.Get("/health", healthCheck)

	// Initialize repositories
	pRepo := productRepo.NewRepository(config.DB)
	uRepo := userRepo.NewRepository(config.DB)

	// Initialize services
	pSvc := productSvc.NewService(pRepo)
	uSvc := userSvc.NewService(uRepo)
	aSvc := authSvc.NewService(uRepo)

	// Initialize handlers
	pHdl := productHandler.NewProductHandler(pSvc)
	uHdl := userHandler.NewUserHandler(uSvc)
	aHdl := authHandler.NewHandler(aSvc)

	// Start cleanup service for temp files
	cleanup := cleanupSvc.NewService()
	cleanup.Start()
	defer cleanup.Stop()

	// Setup routes
	routes.SetupRoutes(app, aHdl, pHdl, uHdl)

	// Start server with graceful shutdown
	port := config.GetEnv("APP_PORT", "4000")
	go func() {
		log.Printf("Server starting on port %s", port)
		log.Printf("Swagger UI: http://localhost:%s/swagger/index.html", port)
		if err := app.Listen(":" + port); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	if err := app.Shutdown(); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

func healthCheck(c *fiber.Ctx) error {
	dbStatus := "connected"
	if sqlDB, err := config.DB.DB(); err != nil || sqlDB.Ping() != nil {
		dbStatus = "disconnected"
	}

	redisStatus := "connected"
	if !config.IsRedisConnected() {
		redisStatus = "disconnected"
	}

	status := "healthy"
	statusCode := fiber.StatusOK
	if dbStatus == "disconnected" {
		status = "degraded"
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status": status,
		"services": fiber.Map{
			"database": dbStatus,
			"redis":    redisStatus,
		},
	})
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"status":  "error",
		"code":    code,
		"message": message,
	})
}
