package constants

import "time"

const (
	// TODO :: Move these to a config file or database table so they can be updated without code changes.
	// LoanDefaultInterestRatePct is interpreted as a MONTHLY flat rate (percent).
	LoanDefaultInterestRatePct = 1.5
	LoanMinAmount              = 500_000.0
	LoanMaxAmount              = 10_000_000.0
	LoanMaxTenorMonths         = 120
	LoanUserConfirmationWindow = 24 * time.Hour
)
