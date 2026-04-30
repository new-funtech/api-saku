package services

import (
	"context"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanApprovalService interface {
	GetPending(ctx context.Context, role string, bujpID *uuid.UUID, page, limit int) ([]model.LoanApprovalResponse, int64, error)
	GetByLoan(ctx context.Context, loanID uuid.UUID) ([]model.LoanApprovalResponse, error)
	Process(ctx context.Context, approvalID uuid.UUID, approverID uuid.UUID, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error)
	ProcessByLoan(ctx context.Context, loanID uuid.UUID, approverID uuid.UUID, role string, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error)
	Disburse(ctx context.Context, loanID uuid.UUID) (*model.LoanResponse, error)
	ResolveUserBujpID(ctx context.Context, userID uuid.UUID) uuid.UUID
	ResolveUserPersonnelID(ctx context.Context, userID uuid.UUID) uuid.UUID
	// LoadLoanScope returns the (BujpID, PersonnelID) pair of a loan so the
	// handler can authorize before exposing approvals or mutating state.
	LoadLoanScope(ctx context.Context, loanID uuid.UUID) (bujpID *uuid.UUID, personnelID uuid.UUID, err error)
}

type loanApprovalServiceImpl struct {
	approvalRepo    repository.LoanApprovalRepository
	loanRepo        repository.LoanRepository
	installmentRepo repository.LoanInstallmentRepository
	loanService     *loanServiceImpl
}

func NewLoanApprovalService(
	approvalRepo repository.LoanApprovalRepository,
	loanRepo repository.LoanRepository,
	installmentRepo repository.LoanInstallmentRepository,
	loanSvc LoanService,
) LoanApprovalService {
	impl, _ := loanSvc.(*loanServiceImpl)
	return &loanApprovalServiceImpl{
		approvalRepo:    approvalRepo,
		loanRepo:        loanRepo,
		installmentRepo: installmentRepo,
		loanService:     impl,
	}
}

func (s *loanApprovalServiceImpl) GetPending(ctx context.Context, role string, bujpID *uuid.UUID, page, limit int) ([]model.LoanApprovalResponse, int64, error) {
	items, total, err := s.approvalRepo.FindPendingForRole(ctx, role, bujpID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.LoanApprovalResponse, len(items))
	for i := range items {
		out[i] = ToLoanApprovalResponse(ctx, &items[i])
	}
	return out, total, nil
}

func (s *loanApprovalServiceImpl) ResolveUserBujpID(ctx context.Context, userID uuid.UUID) uuid.UUID {
	if s.loanService == nil {
		return uuid.Nil
	}
	return s.loanService.ResolveUserBujpID(ctx, userID)
}

func (s *loanApprovalServiceImpl) ResolveUserPersonnelID(ctx context.Context, userID uuid.UUID) uuid.UUID {
	if s.loanService == nil {
		return uuid.Nil
	}
	return s.loanService.ResolveUserPersonnelID(ctx, userID)
}

func (s *loanApprovalServiceImpl) LoadLoanScope(ctx context.Context, loanID uuid.UUID) (*uuid.UUID, uuid.UUID, error) {
	loan, err := s.loanRepo.FindByID(ctx, loanID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if loan == nil {
		return nil, uuid.Nil, gorm.ErrRecordNotFound
	}
	return loan.BujpID, loan.PersonnelID, nil
}

func (s *loanApprovalServiceImpl) GetByLoan(ctx context.Context, loanID uuid.UUID) ([]model.LoanApprovalResponse, error) {
	items, err := s.approvalRepo.FindByLoanID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	out := make([]model.LoanApprovalResponse, len(items))
	for i := range items {
		out[i] = ToLoanApprovalResponse(ctx, &items[i])
	}
	return out, nil
}

func (s *loanApprovalServiceImpl) Process(ctx context.Context, approvalID uuid.UUID, approverID uuid.UUID, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error) {
	approval, err := s.approvalRepo.FindByID(ctx, approvalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("approval not found")
		}
		return nil, err
	}
	if approval.Status != model.LoanApprovalStatusPending {
		return nil, errors.New("approval already processed")
	}
	loan, err := s.loanRepo.FindByID(ctx, approval.LoanID)
	if err != nil {
		return nil, errors.New("loan not found")
	}

	// Ensure sequential processing (lower level first)
	if approval.ApprovalLevel == model.LoanApprovalLevelPusat {
		approvals, _ := s.approvalRepo.FindByLoanID(ctx, loan.ID)
		for _, a := range approvals {
			if a.ApprovalLevel == model.LoanApprovalLevelBujp && a.Status != model.LoanApprovalStatusApproved {
				return nil, errors.New("BUJP approval must be completed first")
			}
		}
	}

	now := time.Now()
	approval.ApproverID = &approverID
	approval.ReviewedAt = &now
	approval.Notes = req.Notes
	approval.ApprovedAmount = req.ApprovedAmount
	approval.ApprovedTenor = req.ApprovedTenor

	if req.Action == "reject" {
		approval.Status = model.LoanApprovalStatusRejected
		if err := s.approvalRepo.Update(ctx, approval); err != nil {
			return nil, err
		}
		loan.Status = model.LoanStatusRejected
		loan.RejectedAt = &now
		loan.RejectionReason = req.Notes
		if err := s.loanRepo.Update(ctx, loan); err != nil {
			return nil, err
		}
	} else {
		approval.Status = model.LoanApprovalStatusApproved
		if err := s.approvalRepo.Update(ctx, approval); err != nil {
			return nil, err
		}
		// Update loan status
		if approval.ApprovalLevel == model.LoanApprovalLevelBujp {
			loan.Status = model.LoanStatusApprovedBujp
			loan.ApprovedAt = &now
		} else {
			loan.ApprovedPusatAt = &now
			if req.ApprovedAmount != nil {
				loan.ApprovedAmount = req.ApprovedAmount
			}
			if req.ApprovedTenor != nil {
				loan.ApprovedTenor = req.ApprovedTenor
			}
			finalAmount := loan.LoanAmount
			if loan.ApprovedAmount != nil && *loan.ApprovedAmount > 0 {
				finalAmount = *loan.ApprovedAmount
			}
			finalTenor := loan.TenorMonths
			if loan.ApprovedTenor != nil && *loan.ApprovedTenor > 0 {
				finalTenor = *loan.ApprovedTenor
				loan.TenorMonths = *loan.ApprovedTenor
			}
			monthly, total := calculateLoanDetails(finalAmount, loan.InterestRate, finalTenor)
			loan.MonthlyInstallment = monthly
			loan.TotalRepayment = total
			loan.Status = model.LoanStatusPendingUserConfirmation
			deadline := now.Add(constants.LoanUserConfirmationWindow)
			loan.UserConfirmationDeadline = &deadline
		}
		if err := s.loanRepo.Update(ctx, loan); err != nil {
			return nil, err
		}
	}
	r := ToLoanApprovalResponse(ctx, approval)
	return &r, nil
}

// ProcessByLoan resolves the next pending approval for a loan that matches the
// caller's role and processes it. This lets the admin client call
// /loan-approvals/{loan_id}/approve|reject without having to know the
// approval row UUID.
func (s *loanApprovalServiceImpl) ProcessByLoan(ctx context.Context, loanID uuid.UUID, approverID uuid.UUID, role string, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error) {
	approvals, err := s.approvalRepo.FindByLoanID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if len(approvals) == 0 {
		return nil, errors.New("no approval records for this loan")
	}
	expectedLevel := model.LoanApprovalLevelBujp
	if role == "admin" || role == "super_admin" {
		expectedLevel = model.LoanApprovalLevelPusat
	}
	var target *model.LoanApproval
	// Prefer the pending one matching role; fall back to first pending.
	for i := range approvals {
		a := &approvals[i]
		if a.Status == model.LoanApprovalStatusPending && a.ApprovalLevel == expectedLevel {
			target = a
			break
		}
	}
	if target == nil {
		for i := range approvals {
			a := &approvals[i]
			if a.Status == model.LoanApprovalStatusPending {
				target = a
				break
			}
		}
	}
	if target == nil {
		return nil, errors.New("no pending approval available for this loan")
	}
	return s.Process(ctx, target.ID, approverID, req)
}

func (s *loanApprovalServiceImpl) Disburse(ctx context.Context, loanID uuid.UUID) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, loanID)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusPendingUserConfirmation && loan.Status != model.LoanStatusApprovedPusat {
		return nil, errors.New("loan is not ready to be disbursed")
	}
	if s.loanService == nil {
		return nil, errors.New("loan service unavailable")
	}
	if err := s.loanService.processDisbursement(ctx, loan); err != nil {
		return nil, err
	}
	if err := s.loanRepo.Update(ctx, loan); err != nil {
		return nil, err
	}
	r := ToLoanResponse(loan)
	return &r, nil
}

// === Mappers ===

func ToLoanApprovalResponse(ctx context.Context, a *model.LoanApproval) model.LoanApprovalResponse {
	levelName := a.ApprovalLevelName
	switch a.ApprovalLevel {
	case model.LoanApprovalLevelBujp:
		levelName = "Admin BUJP"
	case model.LoanApprovalLevelPusat:
		levelName = "Admin BUJP Pusat"
	}
	r := model.LoanApprovalResponse{
		ID:                a.ID,
		LoanID:            a.LoanID,
		ApprovalLevel:     a.ApprovalLevel,
		ApprovalLevelName: levelName,
		ApproverID:        a.ApproverID,
		Status:            a.Status,
		Notes:             a.Notes,
		ApprovedAmount:    a.ApprovedAmount,
		ApprovedTenor:     a.ApprovedTenor,
		ReviewedAt:        a.ReviewedAt,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
	if a.Approver != nil {
		u := toUserResponse(*a.Approver)
		r.Approver = &u
	}
	if a.Loan != nil {
		loan := ToLoanResponse(a.Loan)
		// Presign loan document keys so admin clients (approvals queue)
		// can render document thumbnails without falling back to raw
		// object keys (which the browser then resolves against the
		// current page URL — yielding 404s like /LOANS/{id}/...jpg).
		presignLoanDocs(ctx, &loan)
		r.Loan = &loan
	}
	return r
}
