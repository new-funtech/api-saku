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

	// Overtime pay (general formula, after Kepmenakertrans 102/2004): the hourly wage is the monthly
	// base salary / 173; per day the first overtime hour pays 1.5x and every further hour 2x.
	PayrollMonthlyWorkHours     = 173.0
	OvertimeFirstHourMultiplier = 1.5
	OvertimeNextHourMultiplier  = 2.0
	OvertimeSalaryComponentCode = "LEMBUR"
	OvertimeSalaryComponentName = "Upah Lembur"
)
