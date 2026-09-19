package constants

import "time"

const (
	// TODO :: Move these to a config file or database table so they can be updated without code changes.
	// LoanDefaultInterestRatePct is interpreted as a MONTHLY flat rate (percent).
	LoanDefaultInterestRatePct        = 3.0
	LoanMinAmount                     = 500_000.0
	LoanMaxAmount                     = 10_000_000.0
	LoanMaxTenorMonths                = 120
	LoanUserConfirmationWindow        = 24 * time.Hour
	LoanDefaultRegistrationFee        = 25_000.0 // flat, Rupiah
	LoanDefaultProvisiRatePct         = 2.0      // percent of loan_amount
	LoanDefaultPenaltyEarlyPayoff     = 0.0      // Rupiah
	LoanDefaultPenaltyRunningInterest = 0.0      // Rupiah
)
