package routes

import (
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/handlers"
	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func SetupRoutes(
	app *fiber.App,
	authHdl *handlers.AuthHandler,
	productHdl *handlers.ProductHandler,
	userHdl *handlers.UserHandler,
	bujpHdl *handlers.BujpHandler,
	locationHdl *handlers.LocationHandler,
	shiftHdl *handlers.ShiftHandler,
	personnelHdl *handlers.PersonnelHandler,
	assignmentHdl *handlers.AssignmentHandler,
	attendanceHdl *handlers.AttendanceHandler,
	attendanceCorrectionHdl *handlers.AttendanceCorrectionHandler,
	patrolHdl *handlers.PatrolHandler,
	leaveHdl *handlers.LeaveHandler,
	overtimeHdl *handlers.OvertimeHandler,
	salaryComponentHdl *handlers.SalaryComponentHandler,
	payrollHdl *handlers.PayrollHandler,
	monthlyReportHdl *handlers.MonthlyReportHandler,
	uploadHdl *handlers.UploadHandler,
	dashboardHdl *handlers.DashboardHandler,
	loanProductHdl *handlers.LoanProductHandler,
	loanHdl *handlers.LoanHandler,
	loanApprovalHdl *handlers.LoanApprovalHandler,
	loanInstallmentHdl *handlers.LoanInstallmentHandler,
	notificationHdl *handlers.NotificationHandler,
	deviceHdl *handlers.DeviceHandler,
	preferenceHdl *handlers.NotificationPreferenceHandler,
	realtimeHdl *handlers.RealtimeHandler,
	userRepo repository.UserRepository,
	personnelRepo repository.PersonnelRepository,
) {
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Authenticated requests must also resolve the caller's tenant scope
	// (BUJP / personnel) so handlers can apply row-level filtering.
	// One budget per signed-in user across the API, plus tighter ones for uploads and payroll below.
	userLimiter := middleware.RateLimitByUser(240, time.Minute)
	protected := func() []fiber.Handler {
		return []fiber.Handler{
			middleware.AuthRequired(),
			middleware.LoadCallerScope(userRepo, personnelRepo),
			userLimiter,
		}
	}

	// Stricter rate limit on auth endpoints to deter brute-force attempts.
	authLimiter := middleware.RateLimitByIP(10, time.Minute)

	v1.Post("/login", authLimiter, authHdl.Login)
	v1.Post("/register", authLimiter, authHdl.Register)
	v1.Post("/forgot-password", authLimiter, authHdl.ForgotPassword)
	v1.Post("/verify-reset-otp", authLimiter, authHdl.VerifyResetOTP)
	v1.Post("/reset-password", authLimiter, authHdl.ResetPassword)

	// Upload routes (all protected)
	uploadRoutes := v1.Group("/uploads", append(protected(), middleware.RateLimitByUser(40, time.Minute))...)
	uploadRoutes.Post("/presigned-url", uploadHdl.GetPresignedUploadURL)
	uploadRoutes.Post("/presigned-download-url", uploadHdl.GetPresignedDownloadURL)
	uploadRoutes.Post("/direct", uploadHdl.DirectUpload)
	uploadRoutes.Post("/move", uploadHdl.MoveFile)
	uploadRoutes.Delete("/", uploadHdl.DeleteFile)

	// Dashboard routes (all protected)
	dashboardRoutes := v1.Group("/dashboard", protected()...)
	dashboardRoutes.Get("/summary", dashboardHdl.GetSummary)

	// Product routes
	productRoutes := v1.Group("/products")
	productRoutes.Get("/", productHdl.GetAllProducts)
	productRoutes.Get("/:id", productHdl.GetProductByID)
	productRoutes.Post("/", append(protected(), productHdl.CreateProduct)...)
	productRoutes.Post("/upload", append(protected(), productHdl.UploadProductImage)...)
	productRoutes.Put("/:id", append(protected(), productHdl.UpdateProduct)...)
	productRoutes.Delete("/:id", append(protected(), productHdl.DeleteProduct)...)

	// User routes (all protected)
	userRoutes := v1.Group("/users", protected()...)
	userRoutes.Get("/", userHdl.GetAllUsers)
	userRoutes.Get("/profile", userHdl.GetProfile)
	userRoutes.Put("/profile", userHdl.UpdateProfile)
	userRoutes.Post("/change-password", userHdl.ChangePassword)
	userRoutes.Get("/:id", userHdl.GetUserByID)
	userRoutes.Post("/", userHdl.CreateUser)
	userRoutes.Put("/:id", userHdl.UpdateUser)
	userRoutes.Delete("/:id", userHdl.DeleteUser)

	// BUJP routes (all protected)
	bujpRoutes := v1.Group("/bujps", protected()...)
	bujpRoutes.Get("/", bujpHdl.GetAllBujps)
	bujpRoutes.Get("/:id", bujpHdl.GetBujpByID)
	bujpRoutes.Post("/", bujpHdl.CreateBujp)
	bujpRoutes.Put("/:id", bujpHdl.UpdateBujp)
	bujpRoutes.Delete("/:id", bujpHdl.DeleteBujp)

	// Location routes (all protected)
	locationRoutes := v1.Group("/locations", protected()...)
	locationRoutes.Get("/", locationHdl.GetAllLocations)
	locationRoutes.Get("/:id", locationHdl.GetLocationByID)
	locationRoutes.Post("/", locationHdl.CreateLocation)
	locationRoutes.Put("/:id", locationHdl.UpdateLocation)
	locationRoutes.Delete("/:id", locationHdl.DeleteLocation)

	// Shift routes (all protected)
	shiftRoutes := v1.Group("/shifts", protected()...)
	shiftRoutes.Get("/", shiftHdl.GetAllShifts)
	shiftRoutes.Get("/:id", shiftHdl.GetShiftByID)
	shiftRoutes.Post("/", shiftHdl.CreateShift)
	shiftRoutes.Put("/:id", shiftHdl.UpdateShift)
	shiftRoutes.Delete("/:id", shiftHdl.DeleteShift)

	// Personnel routes (all protected)
	personnelRoutes := v1.Group("/personnels", protected()...)
	personnelRoutes.Get("/", personnelHdl.GetAllPersonnels)
	personnelRoutes.Get("/template/download", personnelHdl.DownloadTemplate)
	personnelRoutes.Post("/bulk-import", personnelHdl.BulkImport)
	personnelRoutes.Get("/:id", personnelHdl.GetPersonnelByID)
	personnelRoutes.Post("/", personnelHdl.CreatePersonnel)
	personnelRoutes.Put("/:id", personnelHdl.UpdatePersonnel)
	personnelRoutes.Delete("/:id", personnelHdl.DeletePersonnel)

	// Personnel routes alias (singular form to match Laravel backend)
	personnelAliasRoutes := v1.Group("/personnel", protected()...)
	personnelAliasRoutes.Get("/", personnelHdl.GetAllPersonnels)
	personnelAliasRoutes.Get("/template/download", personnelHdl.DownloadTemplate)
	personnelAliasRoutes.Post("/bulk-import", personnelHdl.BulkImport)
	personnelAliasRoutes.Get("/:id", personnelHdl.GetPersonnelByID)
	personnelAliasRoutes.Post("/", personnelHdl.CreatePersonnel)
	personnelAliasRoutes.Put("/:id", personnelHdl.UpdatePersonnel)
	personnelAliasRoutes.Delete("/:id", personnelHdl.DeletePersonnel)

	// Assignment routes (all protected)
	assignmentRoutes := v1.Group("/assignments", protected()...)
	assignmentRoutes.Get("/", assignmentHdl.GetAll)
	assignmentRoutes.Get("/:id", assignmentHdl.GetByID)
	assignmentRoutes.Post("/", assignmentHdl.Create)
	assignmentRoutes.Put("/:id", assignmentHdl.Update)
	assignmentRoutes.Delete("/:id", assignmentHdl.Delete)

	// Attendance routes (all protected)
	attendanceRoutes := v1.Group("/attendances", protected()...)
	attendanceRoutes.Get("/", attendanceHdl.GetAll)
	attendanceRoutes.Get("/summary", attendanceHdl.Summary)
	attendanceRoutes.Put("/checkout", attendanceHdl.Checkout)
	attendanceRoutes.Get("/status/:key", attendanceHdl.QueueStatus)
	attendanceRoutes.Get("/:id", attendanceHdl.GetByID)
	attendanceRoutes.Post("/", attendanceHdl.Create)
	// Editing or deleting someone's attendance is an admin action, limited to the admin's own company.
	attendanceAdmin := middleware.RequireRoles("super_admin", "admin", "company_admin")
	attendanceRoutes.Put("/:id", attendanceAdmin, attendanceHdl.Update)
	attendanceRoutes.Delete("/:id", attendanceAdmin, attendanceHdl.Delete)

	// Attendance Correction routes (all protected)
	attendanceCorrectionRoutes := v1.Group("/attendance-corrections", protected()...)
	attendanceCorrectionRoutes.Get("/pending", attendanceCorrectionHdl.Pending)
	attendanceCorrectionRoutes.Get("/", attendanceCorrectionHdl.GetAll)
	attendanceCorrectionRoutes.Get("/:id", attendanceCorrectionHdl.GetByID)
	attendanceCorrectionRoutes.Post("/", attendanceCorrectionHdl.Create)
	// Only admin roles decide leave and correction requests.
	requestReviewer := middleware.RequireRoles("super_admin", "admin", "company_admin")
	attendanceCorrectionRoutes.Post("/:id/approve", requestReviewer, attendanceCorrectionHdl.Approve)
	attendanceCorrectionRoutes.Post("/:id/reject", requestReviewer, attendanceCorrectionHdl.Reject)
	attendanceCorrectionRoutes.Put("/:id", attendanceCorrectionHdl.Update)
	attendanceCorrectionRoutes.Delete("/:id", attendanceCorrectionHdl.Delete)

	// Patrol routes (all protected)
	patrolRoutes := v1.Group("/patrols", protected()...)
	patrolRoutes.Get("/", patrolHdl.GetAll)
	patrolRoutes.Get("/:id", patrolHdl.GetByID)
	patrolRoutes.Post("/", patrolHdl.Create)
	patrolRoutes.Put("/:id", patrolHdl.Update)
	patrolRoutes.Post("/:id/validate", patrolHdl.Validate)
	patrolRoutes.Delete("/:id", patrolHdl.Delete)

	// Leave routes (all protected)
	leaveRoutes := v1.Group("/leaves", protected()...)
	leaveRoutes.Get("/pending", leaveHdl.Pending)
	leaveRoutes.Get("/", leaveHdl.GetAll)
	leaveRoutes.Get("/:id", leaveHdl.GetByID)
	leaveRoutes.Post("/", leaveHdl.Create)
	leaveRoutes.Post("/:id/approve", requestReviewer, leaveHdl.Approve)
	leaveRoutes.Post("/:id/reject", requestReviewer, leaveHdl.Reject)
	leaveRoutes.Put("/:id", leaveHdl.Update)
	leaveRoutes.Delete("/:id", leaveHdl.Delete)

	// Overtime routes: staff file and manage their own; admins decide.
	overtimeRoutes := v1.Group("/overtimes", protected()...)
	overtimeAdmin := middleware.RequireRoles("super_admin", "admin", "company_admin")
	overtimeStaff := middleware.RequireRoles("guard", "supervisor")
	overtimeRoutes.Get("/pending", overtimeAdmin, overtimeHdl.Pending)
	overtimeRoutes.Get("/", overtimeHdl.GetAll)
	overtimeRoutes.Get("/:id", overtimeHdl.GetByID)
	overtimeRoutes.Post("/", overtimeStaff, overtimeHdl.Create)
	overtimeRoutes.Put("/:id", overtimeStaff, overtimeHdl.Update)
	overtimeRoutes.Delete("/:id", overtimeStaff, overtimeHdl.Delete)
	overtimeRoutes.Post("/:id/approve", overtimeAdmin, overtimeHdl.Approve)
	overtimeRoutes.Post("/:id/reject", overtimeAdmin, overtimeHdl.Reject)

	// Device (FCM) tokens: the app registers on sign-in and removes the token on sign-out.
	deviceRoutes := v1.Group("/devices", protected()...)
	deviceRoutes.Post("/token", deviceHdl.RegisterToken)
	deviceRoutes.Delete("/token", deviceHdl.UnregisterToken)

	// Salary Component routes (all protected)
	salaryComponentRoutes := v1.Group("/salary-components", protected()...)
	salaryComponentRoutes.Get("/", salaryComponentHdl.GetAll)
	salaryComponentRoutes.Get("/:id", salaryComponentHdl.GetByID)
	salaryComponentRoutes.Post("/", salaryComponentHdl.Create)
	salaryComponentRoutes.Put("/:id", salaryComponentHdl.Update)
	salaryComponentRoutes.Delete("/:id", salaryComponentHdl.Delete)

	// Payroll routes (all protected)
	payrollRoutes := v1.Group("/payrolls", protected()...)
	payrollRoutes.Get("/", payrollHdl.GetAll)
	payrollRoutes.Get("/:id", payrollHdl.GetByID)
	// Only admins may create or change payroll; staff can only read their own slip.
	payrollAdmin := middleware.RequireRoles("super_admin", "admin", "company_admin")
	payrollRoutes.Post("/generate", payrollAdmin, middleware.RateLimitByUser(6, time.Minute), payrollHdl.Generate)
	payrollRoutes.Post("/", payrollAdmin, payrollHdl.Create)
	payrollRoutes.Put("/:id", payrollAdmin, payrollHdl.Update)
	payrollRoutes.Post("/:id/update-payment-status", payrollAdmin, payrollHdl.UpdatePaymentStatus)
	payrollRoutes.Delete("/:id", payrollAdmin, payrollHdl.Delete)

	// Monthly Report routes (all protected)
	monthlyReportRoutes := v1.Group("/monthly-reports", protected()...)
	monthlyReportRoutes.Get("/", monthlyReportHdl.GetAll)
	monthlyReportRoutes.Post("/generate", monthlyReportHdl.Generate)
	monthlyReportRoutes.Get("/:id", monthlyReportHdl.GetByID)
	monthlyReportRoutes.Post("/", monthlyReportHdl.Create)
	monthlyReportRoutes.Put("/:id", monthlyReportHdl.Update)
	monthlyReportRoutes.Delete("/:id", monthlyReportHdl.Delete)

	// Loan Product routes (all protected)
	loanProductRoutes := v1.Group("/loan-products", protected()...)
	loanProductRoutes.Get("/", loanProductHdl.GetAll)
	loanProductRoutes.Get("/active", loanProductHdl.GetActive)
	loanProductRoutes.Get("/:id", loanProductHdl.GetByID)
	loanProductRoutes.Post("/", loanProductHdl.Create)
	loanProductRoutes.Put("/:id", loanProductHdl.Update)
	loanProductRoutes.Delete("/:id", loanProductHdl.Delete)

	// Loan routes (all protected)
	loanRoutes := v1.Group("/loans", protected()...)
	loanRoutes.Get("/", loanHdl.GetAll)
	loanRoutes.Get("/my-summary", loanInstallmentHdl.MySummary)
	loanRoutes.Get("/admin-summary", middleware.RequireRoles("super_admin", "admin", "company_admin"), loanInstallmentHdl.AdminSummary)
	loanRoutes.Get("/statistics", loanHdl.Statistics)
	loanRoutes.Get("/history", loanHdl.History)
	loanRoutes.Get("/history/:personnel_id", loanHdl.History)
	loanRoutes.Get("/:id", loanHdl.GetByID)
	loanRoutes.Get("/:loan_id/installments", loanInstallmentHdl.ByLoan)
	loanRoutes.Post("/", loanHdl.Create)
	loanRoutes.Post("/:id/submit", loanHdl.Submit)
	loanRoutes.Post("/:id/cancel", loanHdl.Cancel)
	loanRoutes.Post("/:id/user-confirmation", loanHdl.UserConfirm)
	loanRoutes.Put("/:id", loanHdl.Update)
	loanRoutes.Delete("/:id", loanHdl.Delete)

	// Loan Approval routes (all protected)
	loanApprovalRoutes := v1.Group("/loan-approvals", protected()...)
	loanApprovalRoutes.Get("/pending", loanApprovalHdl.Pending)
	loanApprovalRoutes.Get("/loan/:loan_id", loanApprovalHdl.ByLoan)
	loanApprovalRoutes.Post("/:id/process", loanApprovalHdl.Process)
	loanApprovalRoutes.Post("/:loan_id/approve", loanApprovalHdl.ApproveByLoan)
	loanApprovalRoutes.Post("/:loan_id/reject", loanApprovalHdl.RejectByLoan)
	loanApprovalRoutes.Post("/loan/:loan_id/disburse", loanApprovalHdl.Disburse)

	// Loan Installment routes (all protected)
	loanInstallmentRoutes := v1.Group("/loan-installments", protected()...)
	loanInstallmentRoutes.Post("/:id/pay", loanInstallmentHdl.Pay)
	loanInstallmentRoutes.Post("/mark-overdue", loanInstallmentHdl.MarkOverdue)
	notificationRoutes := v1.Group("/notifications", protected()...)
	notificationRoutes.Get("/", notificationHdl.GetAll)
	notificationRoutes.Get("/preferences", preferenceHdl.Get)
	notificationRoutes.Put("/preferences", preferenceHdl.Update)
	notificationRoutes.Get("/unread-count", notificationHdl.GetUnreadCount)
	notificationRoutes.Post("/read-all", notificationHdl.MarkAllAsRead)
	notificationRoutes.Post("/:id/read", notificationHdl.MarkAsRead)
	notificationRoutes.Delete("/clear-read", notificationHdl.ClearRead)
	notificationRoutes.Delete("/:id", notificationHdl.Delete)

	app.Use("/ws/notifications", realtimeHdl.UpgradeAuth)
	app.Get("/ws/notifications", websocket.New(realtimeHdl.Handle))
}
