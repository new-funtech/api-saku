package constants

// Success Messages
const (
	SuccessGetProducts   = "Successfully retrieved products"
	SuccessGetProduct    = "Successfully retrieved product"
	SuccessCreateProduct = "Successfully created product"
	SuccessUpdateProduct = "Successfully updated product"
	SuccessDeleteProduct = "Successfully deleted product"
	SuccessUploadImage   = "Image uploaded successfully"

	SuccessGetUsers   = "Successfully retrieved users"
	SuccessGetUser    = "Successfully retrieved user"
	SuccessCreateUser = "Successfully created user"
	SuccessUpdateUser = "Successfully updated user"
	SuccessDeleteUser = "Successfully deleted user"

	SuccessLogin    = "Login successful"
	SuccessRegister = "Registration successful"

	SuccessForgotPasswordSent = "Kode OTP telah dikirim ke email Anda"
	SuccessVerifyResetOTP     = "OTP valid, silakan atur password baru"
	SuccessResetPassword      = "Password berhasil direset, silakan login"
	ErrInvalidOTP             = "OTP tidak valid atau sudah kedaluwarsa"
	ErrInvalidResetToken      = "Token reset tidak valid atau sudah kedaluwarsa"
	ErrEmailNotRegistered     = "Email tidak terdaftar"
	ErrEmailQueueFailed       = "Gagal mengantrikan email, silakan coba lagi"

	ErrInvalidRequest = "Invalid request body"
	ErrInternalServer = "Internal server error"
	ErrInvalidUUID    = "Invalid UUID format"

	ErrProductNotFound = "Product not found"
	ErrImageRequired   = "Image file is required"
	ErrInvalidFileType = "Invalid file type. Allowed: images (jpeg, png, gif, webp), documents (pdf, doc, docx, xls, xlsx)"
	ErrFileTooLarge    = "File size too large. Maximum size is 10MB"
	ErrUploadFailed    = "Failed to upload image"
	ErrPreviewFailed   = "Failed to generate preview URL"

	ErrUserNotFound = "User not found"
	ErrEmailExists  = "Email already exists"

	ErrInvalidCredentials = "Invalid email or password"
	ErrUnauthorized       = "Authorization header is required"
	ErrInvalidToken       = "Invalid or expired token"

	SuccessDeleteNotification = "Successfully deleted notification"

	SuccessGeneratePresignedURL = "Presigned URL generated successfully"
	SuccessDeleteFile           = "File deleted successfully"
	SuccessMoveFile             = "File moved successfully"

	ErrGeneratePresignedURL = "Failed to generate presigned URL"
	ErrDeleteFailed         = "Failed to delete file"
	ErrMoveFailed           = "Failed to move file"

	MaxUploadSize   = 10 * 1024 * 1024 // 10MB
	MaxImageSize    = 5 * 1024 * 1024  // 5MB for images
	MaxDocumentSize = 10 * 1024 * 1024 // 10MB for documents
)

// User Roles
const (
	RoleSuperAdmin   = "super_admin"
	RoleCompanyAdmin = "company_admin"
	RoleSupervisor   = "supervisor"
	RoleGuard        = "guard"
)

// User Status
const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
)

// Personnel Status
const (
	PersonnelStatusActive   = "active"
	PersonnelStatusOnLeave  = "on_leave"
	PersonnelStatusInactive = "inactive"
	PersonnelStatusResigned = "resigned"
)

// Personnel Gender
const (
	GenderMale   = "M"
	GenderFemale = "F"
)

// Assignment Status
const (
	AssignmentStatusActive    = "active"
	AssignmentStatusCompleted = "completed"
	AssignmentStatusCancelled = "cancelled"
)

// Attendance Status
const (
	AttendanceStatusPresent = "present"
	AttendanceStatusAbsent  = "absent"
	AttendanceStatusLeave   = "leave"
	AttendanceStatusSick    = "sick"
	AttendanceStatusHoliday = "holiday"
)

// Attendance Correction Status
const (
	CorrectionStatusPending  = "pending"
	CorrectionStatusApproved = "approved"
	CorrectionStatusRejected = "rejected"
)

// Attendance Correction Type
const (
	CorrectionTypeCheckIn  = "checkin"
	CorrectionTypeCheckOut = "checkout"
	CorrectionTypeBoth     = "both"
)

// Patrol Status
const (
	PatrolStatusScheduled = "scheduled"
	PatrolStatusOngoing   = "ongoing"
	PatrolStatusCompleted = "completed"
	PatrolStatusCancelled = "cancelled"
)

// Leave Type
const (
	LeaveTypeAnnual    = "annual"
	LeaveTypeSick      = "sick"
	LeaveTypeUnpaid    = "unpaid"
	LeaveTypeMaternity = "maternity"
	LeaveTypePaternity = "paternity"
	LeaveTypeOther     = "other"
)

// Leave Status
const (
	LeaveStatusPending  = "pending"
	LeaveStatusApproved = "approved"
	LeaveStatusRejected = "rejected"
)

// Salary Component Type
const (
	ComponentTypeAllowance = "allowance"
	ComponentTypeDeduction = "deduction"
)

// Salary Component Calculation Method
const (
	CalculationMethodFixed      = "fixed"
	CalculationMethodPercentage = "percentage"
)

// Payroll Payment Status
const (
	PaymentStatusPending   = "pending"
	PaymentStatusPaid      = "paid"
	PaymentStatusCancelled = "cancelled"
)

// Payroll Payment Method
const (
	PaymentMethodCash         = "cash"
	PaymentMethodBankTransfer = "bank_transfer"
	PaymentMethodCheck        = "check"
)

// Common Status
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Date Formats
const (
	DateFormatYYYYMMDD = "2006-01-02"
	DateFormatYYYYMM   = "2006-01"
	TimeFormatRFC3339  = "2006-01-02T15:04:05Z07:00"
)

// Pagination Defaults
const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
)

// Default Values
const (
	DefaultAttendanceRadius = 100 // meters
	DefaultGuardsNeeded     = 1
)
