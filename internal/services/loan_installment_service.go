package services

import (
	"context"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanInstallmentService interface {
	GetByLoan(ctx context.Context, loanID uuid.UUID) ([]model.LoanInstallmentResponse, error)
	Pay(ctx context.Context, id uuid.UUID, req *model.LoanInstallmentPaymentRequest) (*model.LoanInstallmentResponse, error)
	MarkOverdue(ctx context.Context) (int64, error)
	DueStats(ctx context.Context, bujpID uuid.UUID, now time.Time) (overdue int64, overdueAmount float64, dueSoon int64, err error)
	DashboardSummary(ctx context.Context, personnelID uuid.UUID, now time.Time) (*model.LoanDashboardSummary, error)
}

type loanInstallmentServiceImpl struct {
	repo     repository.LoanInstallmentRepository
	loanRepo repository.LoanRepository
}

func NewLoanInstallmentService(repo repository.LoanInstallmentRepository, loanRepo repository.LoanRepository) LoanInstallmentService {
	return &loanInstallmentServiceImpl{repo: repo, loanRepo: loanRepo}
}

func (s *loanInstallmentServiceImpl) GetByLoan(ctx context.Context, loanID uuid.UUID) ([]model.LoanInstallmentResponse, error) {
	items, err := s.repo.FindByLoanID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	out := make([]model.LoanInstallmentResponse, len(items))
	for i := range items {
		out[i] = ToLoanInstallmentResponse(&items[i])
	}
	return out, nil
}

func (s *loanInstallmentServiceImpl) Pay(ctx context.Context, id uuid.UUID, req *model.LoanInstallmentPaymentRequest) (*model.LoanInstallmentResponse, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("installment not found")
		}
		return nil, err
	}
	if inst.Status == model.LoanInstallmentStatusPaid || inst.Status == model.LoanInstallmentStatusWaived {
		return nil, errors.New("installment already settled")
	}
	remaining := inst.InstallmentAmount + inst.LateFee - inst.PaidAmount
	amount := req.PaidAmount
	if amount <= 0 {
		amount = remaining
	}
	if amount > remaining {
		amount = remaining
	}

	var paymentDate time.Time
	if req.PaymentDate == "" {
		paymentDate = time.Now()
	} else {
		pd, perr := time.Parse("2006-01-02", req.PaymentDate)
		if perr != nil {
			return nil, errors.New("invalid payment_date format, expected YYYY-MM-DD")
		}
		paymentDate = pd
	}

	method := req.PaymentMethod
	if method == nil || *method == "" {
		defaultMethod := "payroll_deduction"
		method = &defaultMethod
	}

	inst.PaidAmount += amount
	inst.PaymentDate = &paymentDate
	inst.PaymentMethod = method
	inst.TransactionReference = req.TransactionReference
	inst.Notes = req.Notes
	inst.RemainingAmount = inst.InstallmentAmount + inst.LateFee - inst.PaidAmount
	if inst.RemainingAmount <= 0 {
		inst.Status = model.LoanInstallmentStatusPaid
		inst.RemainingAmount = 0
	}
	if err := s.repo.Update(ctx, inst); err != nil {
		return nil, err
	}
	all, _ := s.repo.FindByLoanID(ctx, inst.LoanID)
	allPaid := true
	for _, it := range all {
		if it.Status != model.LoanInstallmentStatusPaid && it.Status != model.LoanInstallmentStatusWaived {
			allPaid = false
			break
		}
	}
	if allPaid {
		now := time.Now()
		_ = s.loanRepo.UpdateColumns(ctx, inst.LoanID, map[string]interface{}{
			"status":       model.LoanStatusCompleted,
			"completed_at": &now,
		})
	}
	r := ToLoanInstallmentResponse(inst)
	return &r, nil
}

func (s *loanInstallmentServiceImpl) MarkOverdue(ctx context.Context) (int64, error) {
	return s.repo.MarkOverdueBefore(ctx, time.Now())
}

// === Mapper ===

func ToLoanInstallmentResponse(i *model.LoanInstallment) model.LoanInstallmentResponse {
	r := model.LoanInstallmentResponse{
		ID:                   i.ID,
		LoanID:               i.LoanID,
		InstallmentNumber:    i.InstallmentNumber,
		DueDate:              utils.FormatDate(i.DueDate),
		PrincipalAmount:      i.PrincipalAmount,
		InterestAmount:       i.InterestAmount,
		InstallmentAmount:    i.InstallmentAmount,
		Status:               i.Status,
		PaidAmount:           i.PaidAmount,
		RemainingAmount:      i.RemainingAmount,
		PaymentMethod:        i.PaymentMethod,
		PayrollID:            i.PayrollID,
		TransactionReference: i.TransactionReference,
		LateDays:             i.LateDays,
		LateFee:              i.LateFee,
		Notes:                i.Notes,
		CreatedAt:            i.CreatedAt,
		UpdatedAt:            i.UpdatedAt,
	}
	if i.PaymentDate != nil {
		s := utils.FormatDate(*i.PaymentDate)
		r.PaymentDate = &s
	}
	return r
}

const loanDueSoonDays = 7

// DashboardSummary condenses the person's loans for the Beranda: what is outstanding, what is
// due next, and which installments are overdue or fall due within the next week.
func (s *loanInstallmentServiceImpl) DashboardSummary(ctx context.Context, personnelID uuid.UUID, now time.Time) (*model.LoanDashboardSummary, error) {
	unpaid, err := s.repo.FindUnpaidByPersonnel(ctx, personnelID)
	if err != nil {
		return nil, err
	}
	counts, err := s.loanRepo.CountsByStatus(ctx, uuid.Nil, personnelID)
	if err != nil {
		return nil, err
	}
	return buildLoanSummary(unpaid, counts, now), nil
}

// buildLoanSummary is the pure part of DashboardSummary.
func buildLoanSummary(unpaid []model.LoanInstallment, counts map[string]int64, now time.Time) *model.LoanDashboardSummary {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	summary := &model.LoanDashboardSummary{DueSoon: []model.LoanDueItem{}}
	loans := map[uuid.UUID]struct{}{}
	for i := range unpaid {
		in := &unpaid[i]
		remaining := in.InstallmentAmount + in.LateFee - in.PaidAmount
		if remaining < 0 {
			remaining = 0
		}
		summary.Outstanding += remaining
		loans[in.LoanID] = struct{}{}

		due := time.Date(in.DueDate.Year(), in.DueDate.Month(), in.DueDate.Day(), 0, 0, 0, 0, now.Location())
		days := int(due.Sub(today).Hours() / 24)
		item := model.LoanDueItem{
			LoanID: in.LoanID, InstallmentID: in.ID, InstallmentNumber: in.InstallmentNumber,
			DueDate: due.Format("2006-01-02"), Amount: remaining, DaysLeft: days, Overdue: days < 0,
		}
		if in.Loan != nil {
			item.LoanNumber = in.Loan.LoanNumber
		}
		if item.Overdue {
			summary.OverdueCount++
		}
		if summary.NextDue == nil {
			next := item
			summary.NextDue = &next
		}
		if days <= loanDueSoonDays && len(summary.DueSoon) < 5 {
			summary.DueSoon = append(summary.DueSoon, item)
		}
	}
	summary.ActiveLoans = len(loans)
	for _, st := range []string{
		model.LoanStatusDraft, model.LoanStatusSubmitted, model.LoanStatusApprovedBujp,
		model.LoanStatusApprovedPusat, model.LoanStatusPendingUserConfirmation,
	} {
		summary.InProcess += int(counts[st])
	}
	summary.NeedsConfirm = int(counts[model.LoanStatusPendingUserConfirmation])
	return summary
}

func (s *loanInstallmentServiceImpl) DueStats(ctx context.Context, bujpID uuid.UUID, now time.Time) (int64, float64, int64, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.repo.DueStats(ctx, bujpID, today, loanDueSoonDays)
}
