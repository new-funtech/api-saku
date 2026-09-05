package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrRejectNotesRequired = errors.New("rejection notes are required")

type LoanApprovalService interface {
	GetPending(ctx context.Context, role string, bujpID *uuid.UUID, page, limit int) ([]model.LoanApprovalResponse, int64, error)
	GetByLoan(ctx context.Context, loanID uuid.UUID) ([]model.LoanApprovalResponse, error)
	Process(ctx context.Context, approvalID uuid.UUID, approverID uuid.UUID, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error)
	ProcessByLoan(ctx context.Context, loanID uuid.UUID, approverID uuid.UUID, role string, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error)
	Disburse(ctx context.Context, loanID uuid.UUID) (*model.LoanResponse, error)
	ResolveUserBujpID(ctx context.Context, userID uuid.UUID) uuid.UUID
	ResolveUserPersonnelID(ctx context.Context, userID uuid.UUID) uuid.UUID
	LoadLoanScope(ctx context.Context, loanID uuid.UUID) (bujpID *uuid.UUID, personnelID uuid.UUID, err error)
}

type loanApprovalServiceImpl struct {
	approvalRepo    repository.LoanApprovalRepository
	loanRepo        repository.LoanRepository
	installmentRepo repository.LoanInstallmentRepository
	loanService     *loanServiceImpl
	notifier        *notifier.Notifier
}

func NewLoanApprovalService(
	approvalRepo repository.LoanApprovalRepository,
	loanRepo repository.LoanRepository,
	installmentRepo repository.LoanInstallmentRepository,
	loanSvc LoanService,
	notif *notifier.Notifier,
) LoanApprovalService {
	impl, _ := loanSvc.(*loanServiceImpl)
	return &loanApprovalServiceImpl{
		approvalRepo:    approvalRepo,
		loanRepo:        loanRepo,
		installmentRepo: installmentRepo,
		loanService:     impl,
		notifier:        notif,
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

	if req.Action == "reject" {
		if req.Notes == nil || strings.TrimSpace(*req.Notes) == "" {
			return nil, ErrRejectNotesRequired
		}
	}
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

	if approval.ApprovalLevel > model.LoanApprovalLevelBujp {
		approvals, _ := s.approvalRepo.FindByLoanID(ctx, loan.ID)
		for _, a := range approvals {
			if a.ApprovalLevel < approval.ApprovalLevel && a.Status != model.LoanApprovalStatusApproved {
				return nil, errors.New("previous approval level must be completed first")
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
	} else {
		approval.Status = model.LoanApprovalStatusApproved
	}

	claimed, err := s.approvalRepo.ClaimAndUpdate(ctx, approval)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, errors.New("approval already processed")
	}

	if req.Action == "reject" {
		loan.Status = model.LoanStatusRejected
		loan.RejectedAt = &now
		loan.RejectionReason = req.Notes
		if err := s.loanRepo.Update(ctx, loan); err != nil {
			return nil, err
		}
	} else {
		if req.ApprovedAmount != nil {
			loan.ApprovedAmount = req.ApprovedAmount
		}
		if approval.ApprovalLevel == model.LoanApprovalLevelBprks && req.ApprovedTenor != nil {
			loan.ApprovedTenor = req.ApprovedTenor
		}
		finalAmount := loan.LoanAmount
		if loan.ApprovedAmount != nil && *loan.ApprovedAmount > 0 {
			finalAmount = *loan.ApprovedAmount
		}
		finalTenor := loan.TenorMonths
		if loan.ApprovedTenor != nil && *loan.ApprovedTenor > 0 {
			finalTenor = *loan.ApprovedTenor
		}
		monthly, total := calculateLoanDetails(finalAmount, loan.InterestRate, finalTenor)
		loan.MonthlyInstallment = monthly
		loan.TotalRepayment = total
		switch approval.ApprovalLevel {
		case model.LoanApprovalLevelBujp:
			loan.Status = model.LoanStatusApprovedBujp
			loan.ApprovedAt = &now
		case model.LoanApprovalLevelPusat:
			loan.Status = model.LoanStatusApprovedPusat
			loan.ApprovedPusatAt = &now
		default: // LoanApprovalLevelBprks
			loan.Status = model.LoanStatusPendingUserConfirmation
			loan.ApprovedBprksAt = &now
			deadline := now.Add(constants.LoanUserConfirmationWindow)
			loan.UserConfirmationDeadline = &deadline
		}
		if err := s.loanRepo.Update(ctx, loan); err != nil {
			return nil, err
		}
	}
	r := ToLoanApprovalResponse(ctx, approval)
	notifyLoan := loan
	if fresh, ferr := s.loanRepo.FindByID(ctx, loan.ID); ferr == nil && fresh != nil {
		notifyLoan = fresh
	}
	switch approval.ApprovalLevel {
	case model.LoanApprovalLevelBujp:
		s.notifier.LoanBujpDecision(ctx, notifyLoan, req.Action != "reject", req.Notes)
	case model.LoanApprovalLevelPusat:
		s.notifier.LoanPusatDecision(ctx, notifyLoan, req.Action != "reject", req.Notes)
	case model.LoanApprovalLevelBprks:
		s.notifier.LoanBprksDecision(ctx, notifyLoan, req.Action != "reject", req.Notes)
	}
	return &r, nil
}

func (s *loanApprovalServiceImpl) ProcessByLoan(ctx context.Context, loanID uuid.UUID, approverID uuid.UUID, role string, req *model.LoanApprovalRequest) (*model.LoanApprovalResponse, error) {
	approvals, err := s.approvalRepo.FindByLoanID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if len(approvals) == 0 {
		return nil, errors.New("no approval records for this loan")
	}
	expectedLevel := model.LoanApprovalLevelBujp
	switch role {
	case "admin", "super_admin":
		expectedLevel = model.LoanApprovalLevelPusat
	case "bprks":
		expectedLevel = model.LoanApprovalLevelBprks
	}
	var target *model.LoanApproval
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

func ToLoanApprovalResponse(ctx context.Context, a *model.LoanApproval) model.LoanApprovalResponse {
	levelName := a.ApprovalLevelName
	switch a.ApprovalLevel {
	case model.LoanApprovalLevelBujp:
		levelName = "Admin Perusahaan"
	case model.LoanApprovalLevelPusat:
		levelName = "Admin Pusat"
	case model.LoanApprovalLevelBprks:
		levelName = "BPRKS"
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
		presignLoanDocs(ctx, &loan)
		r.Loan = &loan
	}
	return r
}
