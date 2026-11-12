package services

import (
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

func TestBuildLoanSummary(t *testing.T) {
	now := time.Date(2026, 11, 12, 15, 0, 0, 0, time.Local)
	loanA, loanB := uuid.New(), uuid.New()
	inst := func(loan uuid.UUID, n int, due string, amount float64, status string) model.LoanInstallment {
		d, _ := time.ParseInLocation("2006-01-02", due, time.Local)
		return model.LoanInstallment{
			ID: uuid.New(), LoanID: loan, InstallmentNumber: n, DueDate: d,
			InstallmentAmount: amount, Status: status, Loan: &model.Loan{LoanNumber: "LN-" + loan.String()[:4]},
		}
	}
	unpaid := []model.LoanInstallment{
		inst(loanA, 1, "2026-11-10", 500_000, model.LoanInstallmentStatusOverdue), // 2 days late
		inst(loanA, 2, "2026-11-15", 500_000, model.LoanInstallmentStatusPending), // in 3 days
		inst(loanB, 1, "2026-12-30", 300_000, model.LoanInstallmentStatusPending), // far away
	}
	counts := map[string]int64{model.LoanStatusSubmitted: 1, model.LoanStatusPendingUserConfirmation: 1, model.LoanStatusActive: 2}
	s := buildLoanSummary(unpaid, counts, now)

	if s.ActiveLoans != 2 || s.Outstanding != 1_300_000 {
		t.Errorf("active=%d outstanding=%v", s.ActiveLoans, s.Outstanding)
	}
	if s.OverdueCount != 1 {
		t.Errorf("overdue = %d", s.OverdueCount)
	}
	if s.NextDue == nil || !s.NextDue.Overdue || s.NextDue.DaysLeft != -2 {
		t.Errorf("next due = %+v", s.NextDue)
	}
	if len(s.DueSoon) != 2 || s.DueSoon[1].DaysLeft != 3 {
		t.Errorf("due soon = %+v", s.DueSoon)
	}
	if s.InProcess != 2 || s.NeedsConfirm != 1 {
		t.Errorf("in process = %d, needs confirm = %d", s.InProcess, s.NeedsConfirm)
	}
}
