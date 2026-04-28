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
	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		return nil, errors.New("invalid payment_date format, expected YYYY-MM-DD")
	}
	inst.PaidAmount += req.PaidAmount
	inst.PaymentDate = &paymentDate
	inst.PaymentMethod = req.PaymentMethod
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
	// If all installments paid -> complete loan
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
