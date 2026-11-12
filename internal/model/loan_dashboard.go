package model

import "github.com/google/uuid"

// LoanDueItem is one unpaid installment, with how many days are left until it is due.
type LoanDueItem struct {
	LoanID            uuid.UUID `json:"loan_id"`
	LoanNumber        string    `json:"loan_number"`
	InstallmentID     uuid.UUID `json:"installment_id"`
	InstallmentNumber int       `json:"installment_number"`
	DueDate           string    `json:"due_date"` // YYYY-MM-DD
	Amount            float64   `json:"amount"`   // still to pay
	DaysLeft          int       `json:"days_left"`
	Overdue           bool      `json:"overdue"`
}

// LoanDashboardSummary is what the Beranda shows about the signed-in person's loans.
type LoanDashboardSummary struct {
	ActiveLoans  int           `json:"active_loans"`
	Outstanding  float64       `json:"outstanding"`
	NextDue      *LoanDueItem  `json:"next_due"`
	DueSoon      []LoanDueItem `json:"due_soon"` // overdue, or due within the next 7 days
	OverdueCount int           `json:"overdue_count"`
	InProcess    int           `json:"in_process"`    // applications not yet active
	NeedsConfirm int           `json:"needs_confirm"` // approved, waiting for the user's answer
}

// LoanAdminSummary is the loan picture an admin sees on the Beranda: applications by status and how
// many installments are overdue or about to fall due.
type LoanAdminSummary struct {
	ByStatus            map[string]int64 `json:"by_status"`
	OverdueInstallments int64            `json:"overdue_installments"`
	OverdueAmount       float64          `json:"overdue_amount"`
	DueSoonInstallments int64            `json:"due_soon_installments"` // due within the next 7 days
}
