package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.LoanResponse, error)
	GetMyHistory(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.LoanResponse, int64, error)
	GetMyHistoryByUser(ctx context.Context, userID uuid.UUID, page, limit int) (*MyHistoryResult, error)
	Create(ctx context.Context, req *model.CreateLoanRequest, actorUserID uuid.UUID, actorRole string) (*model.LoanResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateLoanRequest) (*model.LoanResponse, error)
	Submit(ctx context.Context, id uuid.UUID) (*model.LoanResponse, error)
	Cancel(ctx context.Context, id uuid.UUID, reason *string) (*model.LoanResponse, error)
	UserConfirm(ctx context.Context, id uuid.UUID, action string, reason *string) (*model.LoanResponse, error)
	Statistics(ctx context.Context) (map[string]interface{}, error)
	StatisticsScoped(ctx context.Context, role string, userID uuid.UUID) (map[string]interface{}, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ResolveUserBujpID(ctx context.Context, userID uuid.UUID) uuid.UUID
	ResolveUserPersonnelID(ctx context.Context, userID uuid.UUID) uuid.UUID
}

// MyHistoryResult bundles paginated history items for a user.
type MyHistoryResult struct {
	Items []model.LoanResponse
	Total int64
}

type loanServiceImpl struct {
	loanRepo        repository.LoanRepository
	productRepo     repository.LoanProductRepository
	approvalRepo    repository.LoanApprovalRepository
	installmentRepo repository.LoanInstallmentRepository
	personnelRepo   repository.PersonnelRepository
	userRepo        repository.UserRepository
}

func NewLoanService(
	loanRepo repository.LoanRepository,
	productRepo repository.LoanProductRepository,
	approvalRepo repository.LoanApprovalRepository,
	installmentRepo repository.LoanInstallmentRepository,
	personnelRepo repository.PersonnelRepository,
	userRepo repository.UserRepository,
) LoanService {
	return &loanServiceImpl{
		loanRepo:        loanRepo,
		productRepo:     productRepo,
		approvalRepo:    approvalRepo,
		installmentRepo: installmentRepo,
		personnelRepo:   personnelRepo,
		userRepo:        userRepo,
	}
}

// === Read ===

func (s *loanServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanResponse, int64, error) {
	loans, total, err := s.loanRepo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.LoanResponse, len(loans))
	for i := range loans {
		s.expireIfDeadlinePassed(ctx, &loans[i])
		out[i] = ToLoanResponse(&loans[i])
		presignLoanDocs(ctx, &out[i])
	}
	return out, total, nil
}

func (s *loanServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("loan not found")
		}
		return nil, err
	}
	s.expireIfDeadlinePassed(ctx, loan)
	r := ToLoanResponse(loan)
	presignLoanDocs(ctx, &r)
	return &r, nil
}

// expireIfDeadlinePassed performs lazy auto-cancel for loans stuck in
// pending_user_confirmation past their 24h confirmation deadline. Mutates
// the loan in-place when it transitions so callers see the new status.
// Best-effort: persistence errors are ignored to avoid blocking reads.
func (s *loanServiceImpl) expireIfDeadlinePassed(ctx context.Context, loan *model.Loan) {
	if loan == nil {
		return
	}
	if loan.Status != model.LoanStatusPendingUserConfirmation {
		return
	}
	if loan.UserConfirmationDeadline == nil {
		return
	}
	if !time.Now().After(*loan.UserConfirmationDeadline) {
		return
	}
	loan.Status = model.LoanStatusCancelled
	reason := "Tidak ada konfirmasi user dalam 24 jam"
	loan.RejectionReason = &reason
	now := time.Now()
	loan.RejectedAt = &now
	_ = s.loanRepo.Update(ctx, loan)
}

func (s *loanServiceImpl) GetMyHistory(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.LoanResponse, int64, error) {
	loans, total, err := s.loanRepo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.LoanResponse, len(loans))
	for i := range loans {
		out[i] = ToLoanResponse(&loans[i])
		presignLoanDocs(ctx, &out[i])
	}
	return out, total, nil
}

func (s *loanServiceImpl) GetMyHistoryByUser(ctx context.Context, userID uuid.UUID, page, limit int) (*MyHistoryResult, error) {
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err != nil || personnel == nil {
		return nil, errors.New("personnel not found for current user")
	}
	items, total, err := s.GetMyHistory(ctx, personnel.ID, page, limit)
	if err != nil {
		return nil, err
	}
	return &MyHistoryResult{Items: items, Total: total}, nil
}

// === Write ===

func (s *loanServiceImpl) Create(ctx context.Context, req *model.CreateLoanRequest, actorUserID uuid.UUID, actorRole string) (*model.LoanResponse, error) {
	// Enforce business limits ported from laravel-backend (LoanController@store).
	if req.LoanAmount < constants.LoanMinAmount {
		return nil, fmt.Errorf("jumlah pinjaman minimal Rp %.0f", constants.LoanMinAmount)
	}
	if req.LoanAmount > constants.LoanMaxAmount {
		return nil, fmt.Errorf("jumlah pinjaman maksimal Rp %.0f", constants.LoanMaxAmount)
	}
	if req.TenorMonths > constants.LoanMaxTenorMonths {
		return nil, fmt.Errorf("tenor maksimal %d bulan", constants.LoanMaxTenorMonths)
	}

	// Resolve personnel
	personnelID, err := s.resolvePersonnelID(ctx, req.PersonnelID, actorUserID)
	if err != nil {
		return nil, err
	}
	personnel, err := s.personnelRepo.FindByID(ctx, personnelID)
	if err != nil {
		return nil, errors.New("personnel not found")
	}

	// Resolve interest rate. Optional product lookup (legacy) — otherwise use the
	// rate from the request, defaulting to the flat default per BE business rule.
	interestRate := constants.LoanDefaultInterestRatePct
	var productID *uuid.UUID
	if req.InterestRate != nil {
		interestRate = *req.InterestRate
	}
	if req.LoanProductID != nil && *req.LoanProductID != uuid.Nil {
		product, perr := s.productRepo.FindByID(ctx, *req.LoanProductID)
		if perr != nil {
			return nil, errors.New("loan product not found")
		}
		if !product.IsActive {
			return nil, errors.New("loan product is not active")
		}
		if req.LoanAmount < product.MinAmount || (product.MaxAmount > 0 && req.LoanAmount > product.MaxAmount) {
			return nil, fmt.Errorf("loan amount must be between %.2f and %.2f", product.MinAmount, product.MaxAmount)
		}
		if req.TenorMonths > product.MaxTenor {
			return nil, fmt.Errorf("tenor exceeds product maximum (%d months)", product.MaxTenor)
		}
		if req.InterestRate == nil {
			interestRate = product.InterestRate
		}
		pid := product.ID
		productID = &pid
	}

	// Calculate financial terms (FLAT INTEREST)
	monthly, total := calculateLoanDetails(req.LoanAmount, interestRate, req.TenorMonths)

	loanNumber, err := s.generateLoanNumber(ctx)
	if err != nil {
		return nil, err
	}

	var bujpID *uuid.UUID
	if personnel.BujpID != uuid.Nil {
		bid := personnel.BujpID
		bujpID = &bid
	}

	loan := &model.Loan{
		LoanNumber:         loanNumber,
		PersonnelID:        personnelID,
		BujpID:             bujpID,
		LoanProductID:      productID,
		Purpose:            req.Purpose,
		LoanAmount:         req.LoanAmount,
		InterestRate:       interestRate,
		TenorMonths:        req.TenorMonths,
		MonthlyInstallment: monthly,
		TotalRepayment:     total,
		DeductFromPayroll:  derefBoolDefault(req.DeductFromPayroll, true),
		KtpDocument:        req.KtpDocument,
		NpwpDocument:       req.NpwpDocument,
		SelfieDocument:     req.SelfieDocument,
		SelfieKtpDocument:  req.SelfieKtpDocument,
		PksDocument:        req.PksDocument,
		CollateralDocument: req.CollateralDocument,
		PlacementBujpName:  req.PlacementBujpName,
		PlacementAddress:   req.PlacementAddress,
		PlacementDuration:  req.PlacementDuration,
		Status:             model.LoanStatusDraft,
		CreatedBy:          &actorUserID,
	}

	if err := s.loanRepo.Create(ctx, loan); err != nil {
		return nil, err
	}

	if req.SubmitImmediately {
		if _, err := s.Submit(ctx, loan.ID); err != nil {
			return nil, err
		}
	}

	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateLoanRequest) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusDraft {
		return nil, errors.New("only draft loans can be updated")
	}
	if req.Purpose != nil {
		loan.Purpose = *req.Purpose
	}
	if req.LoanAmount != nil {
		loan.LoanAmount = *req.LoanAmount
	}
	if req.TenorMonths != nil {
		loan.TenorMonths = *req.TenorMonths
	}
	if req.LoanAmount != nil || req.TenorMonths != nil {
		monthly, total := calculateLoanDetails(loan.LoanAmount, loan.InterestRate, loan.TenorMonths)
		loan.MonthlyInstallment = monthly
		loan.TotalRepayment = total
	}
	if req.DeductFromPayroll != nil {
		loan.DeductFromPayroll = *req.DeductFromPayroll
	}
	if req.KtpDocument != nil {
		loan.KtpDocument = req.KtpDocument
	}
	if req.NpwpDocument != nil {
		loan.NpwpDocument = req.NpwpDocument
	}
	if req.SelfieDocument != nil {
		loan.SelfieDocument = req.SelfieDocument
	}
	if req.SelfieKtpDocument != nil {
		loan.SelfieKtpDocument = req.SelfieKtpDocument
	}
	if req.PksDocument != nil {
		loan.PksDocument = req.PksDocument
	}
	if req.CollateralDocument != nil {
		loan.CollateralDocument = req.CollateralDocument
	}
	if err := s.loanRepo.Update(ctx, loan); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) Submit(ctx context.Context, id uuid.UUID) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusDraft {
		return nil, errors.New("only draft loans can be submitted")
	}
	now := time.Now()
	loan.Status = model.LoanStatusSubmitted
	loan.SubmittedAt = &now
	if err := s.loanRepo.Update(ctx, loan); err != nil {
		return nil, err
	}
	if err := s.createApprovalRecords(ctx, loan.ID); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) Cancel(ctx context.Context, id uuid.UUID, reason *string) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status == model.LoanStatusActive ||
		loan.Status == model.LoanStatusDisbursed ||
		loan.Status == model.LoanStatusCompleted {
		return nil, errors.New("loan cannot be cancelled in current status")
	}
	loan.Status = model.LoanStatusCancelled
	loan.RejectionReason = reason
	if err := s.loanRepo.Update(ctx, loan); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) UserConfirm(ctx context.Context, id uuid.UUID, action string, reason *string) (*model.LoanResponse, error) {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusPendingUserConfirmation {
		return nil, errors.New("loan is not pending user confirmation")
	}
	if loan.UserConfirmationDeadline != nil && time.Now().After(*loan.UserConfirmationDeadline) {
		// Auto cancel
		loan.Status = model.LoanStatusCancelled
		r := "User confirmation deadline expired"
		loan.RejectionReason = &r
		_ = s.loanRepo.Update(ctx, loan)
		return nil, errors.New("user confirmation deadline expired; loan cancelled")
	}
	now := time.Now()
	// Normalize action: accept both short and past-tense forms used by clients.
	switch action {
	case "accept", "accepted":
		action = "accept"
	case "decline", "declined", "reject", "rejected":
		action = "decline"
	default:
		return nil, errors.New("invalid action: must be 'accept' or 'decline'")
	}
	loan.UserConfirmedAt = &now
	if action == "decline" {
		loan.Status = model.LoanStatusCancelled
		loan.RejectionReason = reason
	} else {
		// Accept -> proceed to disbursement
		if err := s.processDisbursement(ctx, loan); err != nil {
			return nil, err
		}
	}
	if err := s.loanRepo.Update(ctx, loan); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) Statistics(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	for _, st := range []string{
		model.LoanStatusDraft, model.LoanStatusSubmitted, model.LoanStatusApprovedBujp,
		model.LoanStatusApprovedPusat, model.LoanStatusPendingUserConfirmation,
		model.LoanStatusRejected, model.LoanStatusDisbursed, model.LoanStatusActive,
		model.LoanStatusCompleted, model.LoanStatusCancelled,
	} {
		c, _ := s.loanRepo.CountByStatus(ctx, st)
		stats[st] = c
	}
	return stats, nil
}

func (s *loanServiceImpl) StatisticsScoped(ctx context.Context, role string, userID uuid.UUID) (map[string]interface{}, error) {
	var bujpID, personnelID uuid.UUID
	switch role {
	case "company_admin", "supervisor":
		bujpID = s.ResolveUserBujpID(ctx, userID)
		if bujpID == uuid.Nil {
			// User is not bound to any BUJP → return all-zero stats
			// instead of leaking global numbers.
			return emptyLoanStats(), nil
		}
	case "guard":
		personnelID = s.ResolveUserPersonnelID(ctx, userID)
		if personnelID == uuid.Nil {
			return emptyLoanStats(), nil
		}
	case "super_admin", "admin":
		// no scoping
	default:
		// Unknown role: be safe — return zeros.
		return emptyLoanStats(), nil
	}

	stats := map[string]interface{}{}
	for _, st := range loanStatStatuses() {
		c, _ := s.loanRepo.CountByStatusScoped(ctx, st, bujpID, personnelID)
		stats[st] = c
	}
	return stats, nil
}

func loanStatStatuses() []string {
	return []string{
		model.LoanStatusDraft, model.LoanStatusSubmitted, model.LoanStatusApprovedBujp,
		model.LoanStatusApprovedPusat, model.LoanStatusPendingUserConfirmation,
		model.LoanStatusRejected, model.LoanStatusDisbursed, model.LoanStatusActive,
		model.LoanStatusCompleted, model.LoanStatusCancelled,
	}
}

func emptyLoanStats() map[string]interface{} {
	m := map[string]interface{}{}
	for _, st := range loanStatStatuses() {
		m[st] = int64(0)
	}
	return m
}

func (s *loanServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusDraft && loan.Status != model.LoanStatusCancelled && loan.Status != model.LoanStatusRejected {
		return errors.New("only draft/cancelled/rejected loans can be deleted")
	}
	return s.loanRepo.Delete(ctx, id)
}

// ResolveUserBujpID returns the BUJP UUID for a given user, or uuid.Nil when
// the user is not bound to any BUJP. Used by handlers to scope merchant data.
func (s *loanServiceImpl) ResolveUserBujpID(ctx context.Context, userID uuid.UUID) uuid.UUID {
	if userID == uuid.Nil {
		return uuid.Nil
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil || user.BujpID == nil {
		return uuid.Nil
	}
	return *user.BujpID
}

// ResolveUserPersonnelID returns the personnel UUID linked to the given user.
func (s *loanServiceImpl) ResolveUserPersonnelID(ctx context.Context, userID uuid.UUID) uuid.UUID {
	if userID == uuid.Nil {
		return uuid.Nil
	}
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err != nil || personnel == nil {
		return uuid.Nil
	}
	return personnel.ID
}

// === Internal helpers (also used by approval service) ===

func (s *loanServiceImpl) resolvePersonnelID(ctx context.Context, requested *uuid.UUID, actorUserID uuid.UUID) (uuid.UUID, error) {
	if requested != nil && *requested != uuid.Nil {
		return *requested, nil
	}
	user, err := s.userRepo.FindByID(ctx, actorUserID)
	if err != nil {
		return uuid.Nil, errors.New("authenticated user not found")
	}
	// Try find personnel via user_id linkage
	personnel, err := s.personnelRepo.FindByUserID(ctx, user.ID)
	if err != nil || personnel == nil {
		return uuid.Nil, errors.New("personnel_id is required")
	}
	return personnel.ID, nil
}

func (s *loanServiceImpl) generateLoanNumber(ctx context.Context) (string, error) {
	count, err := s.loanRepo.CountThisMonth(ctx)
	if err != nil {
		return "", err
	}
	now := time.Now()
	return fmt.Sprintf("LN-%04d-%02d-%04d", now.Year(), now.Month(), count+1), nil
}

func (s *loanServiceImpl) createApprovalRecords(ctx context.Context, loanID uuid.UUID) error {
	approvals := []model.LoanApproval{
		{
			LoanID:            loanID,
			ApprovalLevel:     model.LoanApprovalLevelBujp,
			ApprovalLevelName: "Admin BUJP",
			Status:            model.LoanApprovalStatusPending,
		},
		{
			LoanID:            loanID,
			ApprovalLevel:     model.LoanApprovalLevelPusat,
			ApprovalLevelName: "Admin BUJP Pusat",
			Status:            model.LoanApprovalStatusPending,
		},
	}
	return s.approvalRepo.CreateMany(ctx, approvals)
}

// processDisbursement transitions a confirmed loan to disbursed/active and generates installments.
func (s *loanServiceImpl) processDisbursement(ctx context.Context, loan *model.Loan) error {
	now := time.Now()
	loan.DisbursementDate = &now
	first := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local)
	loan.FirstInstallmentDate = &first

	approvedAmount := loan.LoanAmount
	if loan.ApprovedAmount != nil {
		approvedAmount = *loan.ApprovedAmount
	}
	approvedTenor := loan.TenorMonths
	if loan.ApprovedTenor != nil {
		approvedTenor = *loan.ApprovedTenor
	}

	monthlyPrincipal := round2(approvedAmount / float64(approvedTenor))
	// InterestRate is monthly flat (percent). Total interest = principal × rate × months.
	totalInterest := approvedAmount * (loan.InterestRate / 100) * float64(approvedTenor)
	monthlyInterest := round2(totalInterest / float64(approvedTenor))

	installments := make([]model.LoanInstallment, 0, approvedTenor)
	for i := 0; i < approvedTenor; i++ {
		due := first.AddDate(0, i, 0)
		amt := monthlyPrincipal + monthlyInterest
		installments = append(installments, model.LoanInstallment{
			LoanID:            loan.ID,
			InstallmentNumber: i + 1,
			DueDate:           due,
			PrincipalAmount:   monthlyPrincipal,
			InterestAmount:    monthlyInterest,
			InstallmentAmount: amt,
			RemainingAmount:   amt,
			Status:            model.LoanInstallmentStatusPending,
		})
	}
	if err := s.installmentRepo.CreateMany(ctx, installments); err != nil {
		return err
	}
	loan.Status = model.LoanStatusActive
	return nil
}

// === Math ===

// calculateLoanDetails uses FLAT INTEREST with a MONTHLY rate.
// Total Interest = Principal × (MonthlyRate/100) × Tenor
// Total Repayment = Principal + Total Interest
// Monthly Installment = Total Repayment / Tenor
func calculateLoanDetails(principal, monthlyRatePct float64, tenorMonths int) (monthly, total float64) {
	if tenorMonths <= 0 {
		return 0, 0
	}
	totalInterest := principal * (monthlyRatePct / 100) * float64(tenorMonths)
	total = round2(principal + totalInterest)
	monthly = round2(total / float64(tenorMonths))
	return
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// === Mappers ===

// presignLoanDocs fills *DocumentURL fields by presigning the stored object keys.
// Errors are silently ignored — the raw key field remains for fallback.
func presignLoanDocs(ctx context.Context, r *model.LoanResponse) {
	if r == nil {
		return
	}
	pairs := []struct {
		key *string
		dst **string
	}{
		{r.KtpDocument, &r.KtpDocumentURL},
		{r.NpwpDocument, &r.NpwpDocumentURL},
		{r.SelfieDocument, &r.SelfieDocumentURL},
		{r.SelfieKtpDocument, &r.SelfieKtpDocumentURL},
		{r.PksDocument, &r.PksDocumentURL},
		{r.CollateralDocument, &r.CollateralDocumentURL},
	}
	for _, p := range pairs {
		if p.key == nil || *p.key == "" {
			continue
		}
		if url, err := utils.GeneratePresignedURL(ctx, *p.key, time.Hour); err == nil && url != "" {
			u := url
			*p.dst = &u
		}
	}
}

func ToLoanResponse(l *model.Loan) model.LoanResponse {
	r := model.LoanResponse{
		ID:                       l.ID,
		LoanNumber:               l.LoanNumber,
		PersonnelID:              l.PersonnelID,
		BujpID:                   l.BujpID,
		LoanProductID:            l.LoanProductID,
		Purpose:                  l.Purpose,
		LoanAmount:               l.LoanAmount,
		InterestRate:             l.InterestRate,
		TenorMonths:              l.TenorMonths,
		MonthlyInstallment:       l.MonthlyInstallment,
		TotalRepayment:           l.TotalRepayment,
		ApprovedAmount:           l.ApprovedAmount,
		ApprovedTenor:            l.ApprovedTenor,
		DisbursementDate:         dateStrPtr(l.DisbursementDate),
		FirstInstallmentDate:     dateStrPtr(l.FirstInstallmentDate),
		DeductFromPayroll:        l.DeductFromPayroll,
		KtpDocument:              l.KtpDocument,
		NpwpDocument:             l.NpwpDocument,
		SelfieDocument:           l.SelfieDocument,
		SelfieKtpDocument:        l.SelfieKtpDocument,
		PksDocument:              l.PksDocument,
		CollateralDocument:       l.CollateralDocument,
		PlacementCompanyName:     l.PlacementCompanyName,
		PlacementBujpName:        l.PlacementBujpName,
		PlacementAddress:         l.PlacementAddress,
		PlacementDuration:        l.PlacementDuration,
		PlacementLocation:        l.PlacementLocation,
		Status:                   l.Status,
		SubmittedAt:              l.SubmittedAt,
		ApprovedAt:               l.ApprovedAt,
		ApprovedPusatAt:          l.ApprovedPusatAt,
		RejectedAt:               l.RejectedAt,
		RejectionReason:          l.RejectionReason,
		UserConfirmationDeadline: l.UserConfirmationDeadline,
		UserConfirmedAt:          l.UserConfirmedAt,
		CompletedAt:              l.CompletedAt,
		CreatedBy:                l.CreatedBy,
		CreatedAt:                l.CreatedAt,
		UpdatedAt:                l.UpdatedAt,
	}
	if l.Personnel != nil {
		p := ToPersonnelResponse(l.Personnel)
		r.Personnel = &p
	}
	if l.Bujp != nil {
		b := toBujpResponse(l.Bujp)
		r.Bujp = &b
	}
	if l.LoanProduct != nil {
		lp := ToLoanProductResponse(l.LoanProduct)
		r.LoanProduct = &lp
	}
	if len(l.Approvals) > 0 {
		out := make([]model.LoanApprovalResponse, len(l.Approvals))
		for i := range l.Approvals {

			out[i] = ToLoanApprovalResponse(context.Background(), &l.Approvals[i])
		}
		r.Approvals = out
	}
	if len(l.Installments) > 0 {
		out := make([]model.LoanInstallmentResponse, len(l.Installments))
		for i := range l.Installments {
			out[i] = ToLoanInstallmentResponse(&l.Installments[i])
		}
		r.Installments = out
	}
	finalAmount := l.LoanAmount
	if l.ApprovedAmount != nil && *l.ApprovedAmount > 0 {
		finalAmount = *l.ApprovedAmount
	}
	finalTenor := l.TenorMonths
	if l.ApprovedTenor != nil && *l.ApprovedTenor > 0 {
		finalTenor = *l.ApprovedTenor
	}
	if finalAmount != l.LoanAmount || finalTenor != l.TenorMonths {
		monthly, total := calculateLoanDetails(finalAmount, l.InterestRate, finalTenor)
		r.MonthlyInstallment = monthly
		r.TotalRepayment = total
		r.TenorMonths = finalTenor
	}
	return r
}

func dateStrPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := utils.FormatDate(*t)
	return &s
}
