package constants

const (
	// TODO :: Move these to a config file or database table so they can be updated without code changes.
	PayrollPeriodFormat             = "2006-01"
	PayrollDefaultPaymentStatus     = PaymentStatusPending
	SalaryComponentTypeAllowance    = "allowance"
	SalaryComponentTypeDeduction    = "deduction"
	SalaryComponentMethodFixed      = "fixed"
	SalaryComponentMethodPercentage = "percentage"

	AttendanceStatusPermission = "permission"
	PayrollPercentageDivisor   = 100.0
)
