package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/handlers"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/routes"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	cleanupSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/cleanup"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/emailworker"
	"github.com/ganiramadhan/ganipedia/backend/pkg/mailer"

	_ "github.com/ganiramadhan/ganipedia/backend/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
)

// @title           HRMIS API
// @version         1.0
// @description     HRMIS Backend API - Human Resource Management Information System
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@hrmis.com

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

	// Connect to RabbitMQ
	config.ConnectRabbitMQ()

	// Ensure graceful shutdown
	defer config.CloseConnections()

	// Initialize Fiber with production-ready config
	app := fiber.New(fiber.Config{
		AppName:      "HRMIS API v1.0",
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

	// Global rate limiter: 300 req/min per IP. Skips successful health checks.
	app.Use(limiter.New(limiter.Config{
		Max:        300,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/health"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status": "error", "code": fiber.StatusTooManyRequests, "message": "too many requests",
			})
		},
	}))

	// Swagger
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Health check with dependency status
	app.Get("/health", healthCheck)

	// Initialize repositories
	productRepo := repository.NewProductRepository(config.DB)
	userRepo := repository.NewUserRepository(config.DB)
	bujpRepo := repository.NewBujpRepository(config.DB)
	locationRepo := repository.NewLocationRepository(config.DB)
	shiftRepo := repository.NewShiftRepository(config.DB)
	personnelRepo := repository.NewPersonnelRepository(config.DB)
	assignmentRepo := repository.NewAssignmentRepository(config.DB)
	attendanceRepo := repository.NewAttendanceRepository(config.DB)
	attendanceCorrectionRepo := repository.NewAttendanceCorrectionRepository(config.DB)
	patrolRepo := repository.NewPatrolRepository(config.DB)
	leaveRepo := repository.NewLeaveRepository(config.DB)
	salaryComponentRepo := repository.NewSalaryComponentRepository(config.DB)
	payrollRepo := repository.NewPayrollRepository(config.DB)
	monthlyReportRepo := repository.NewMonthlyReportRepository(config.DB)
	loanProductRepo := repository.NewLoanProductRepository(config.DB)
	loanRepo := repository.NewLoanRepository(config.DB)
	loanApprovalRepo := repository.NewLoanApprovalRepository(config.DB)
	loanInstallmentRepo := repository.NewLoanInstallmentRepository(config.DB)
	// notificationRepo := repository.NewNotificationRepository(config.DB) // Notification feature disabled

	// Initialize services
	productSvc := services.NewProductService(productRepo)
	userSvc := services.NewUserService(userRepo, personnelRepo, assignmentRepo, loanRepo)
	authSvc := services.NewAuthService(userRepo, config.RabbitClient, config.GetEnv("RABBITMQ_EMAIL_QUEUE", "email.send"))
	bujpSvc := services.NewBujpService(bujpRepo)
	locationSvc := services.NewLocationService(locationRepo)
	shiftSvc := services.NewShiftService(shiftRepo)
	personnelSvc := services.NewPersonnelService(personnelRepo, loanRepo, userRepo)
	assignmentSvc := services.NewAssignmentService(assignmentRepo)
	attendanceSvc := services.NewAttendanceService(attendanceRepo, personnelRepo, assignmentRepo)
	attendanceCorrectionSvc := services.NewAttendanceCorrectionService(attendanceCorrectionRepo)
	patrolSvc := services.NewPatrolService(patrolRepo, attendanceRepo)
	leaveSvc := services.NewLeaveService(leaveRepo)
	salaryComponentSvc := services.NewSalaryComponentService(salaryComponentRepo)
	payrollSvc := services.NewPayrollService(payrollRepo, personnelRepo, attendanceRepo, salaryComponentRepo)
	monthlyReportSvc := services.NewMonthlyReportService(monthlyReportRepo)
	loanProductSvc := services.NewLoanProductService(loanProductRepo)
	loanSvc := services.NewLoanService(loanRepo, loanProductRepo, loanApprovalRepo, loanInstallmentRepo, personnelRepo, userRepo)
	loanApprovalSvc := services.NewLoanApprovalService(loanApprovalRepo, loanRepo, loanInstallmentRepo, loanSvc)
	loanInstallmentSvc := services.NewLoanInstallmentService(loanInstallmentRepo, loanRepo)
	// notificationSvc := services.NewNotificationService(notificationRepo) // Notification feature disabled
	dashboardSvc := services.NewDashboardService(userRepo, bujpRepo, locationRepo, personnelRepo, assignmentRepo, attendanceRepo, leaveRepo)

	// Initialize handlers
	productHdl := handlers.NewProductHandler(productSvc)
	userHdl := handlers.NewUserHandler(userSvc)
	authHdl := handlers.NewAuthHandler(authSvc)
	bujpHdl := handlers.NewBujpHandler(bujpSvc)
	locationHdl := handlers.NewLocationHandler(locationSvc)
	shiftHdl := handlers.NewShiftHandler(shiftSvc)
	personnelImportSvc := services.NewPersonnelImportService(personnelRepo, userRepo)
	personnelHdl := handlers.NewPersonnelHandler(personnelSvc, personnelImportSvc, userRepo)
	assignmentHdl := handlers.NewAssignmentHandler(assignmentSvc)
	attendanceHdl := handlers.NewAttendanceHandler(attendanceSvc)
	attendanceCorrectionHdl := handlers.NewAttendanceCorrectionHandler(attendanceCorrectionSvc)
	patrolHdl := handlers.NewPatrolHandler(patrolSvc)
	leaveHdl := handlers.NewLeaveHandler(leaveSvc)
	salaryComponentHdl := handlers.NewSalaryComponentHandler(salaryComponentSvc)
	payrollHdl := handlers.NewPayrollHandler(payrollSvc)
	monthlyReportHdl := handlers.NewMonthlyReportHandler(monthlyReportSvc, userRepo)
	loanProductHdl := handlers.NewLoanProductHandler(loanProductSvc)
	loanHdl := handlers.NewLoanHandler(loanSvc)
	loanApprovalHdl := handlers.NewLoanApprovalHandler(loanApprovalSvc)
	loanInstallmentHdl := handlers.NewLoanInstallmentHandler(loanInstallmentSvc, loanSvc)
	// notificationHdl := handlers.NewNotificationHandler(notificationSvc) // Notification feature disabled
	uploadHdl := handlers.NewUploadHandler()
	dashboardHdl := handlers.NewDashboardHandler(dashboardSvc)

	// Start cleanup service for temp files
	cleanup := cleanupSvc.NewService()
	cleanup.Start()
	defer cleanup.Stop()

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	go emailworker.Run(
		workerCtx,
		config.RabbitClient,
		config.GetEnv("RABBITMQ_EMAIL_QUEUE", "email.send"),
		mailer.LoadConfig(),
	)

	// Setup routes
	routes.SetupRoutes(app, authHdl, productHdl, userHdl, bujpHdl, locationHdl, shiftHdl, personnelHdl, assignmentHdl, attendanceHdl, attendanceCorrectionHdl, patrolHdl, leaveHdl, salaryComponentHdl, payrollHdl, monthlyReportHdl, uploadHdl, dashboardHdl, loanProductHdl, loanHdl, loanApprovalHdl, loanInstallmentHdl, userRepo, personnelRepo)

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
