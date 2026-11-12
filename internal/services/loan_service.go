package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type LoanService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.LoanResponse, error)
	GetMyHistory(ctx context.Context, personnelID uuid.UUID, page, limit int, filters map[string]interface{}) ([]model.LoanResponse, int64, error)
	GetMyHistoryByUser(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) (*MyHistoryResult, error)
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
	notifier        *notifier.Notifier
	notificationSvc NotificationService
}

func NewLoanService(
	loanRepo repository.LoanRepository,
	productRepo repository.LoanProductRepository,
	approvalRepo repository.LoanApprovalRepository,
	installmentRepo repository.LoanInstallmentRepository,
	personnelRepo repository.PersonnelRepository,
	userRepo repository.UserRepository,
	notif *notifier.Notifier,
	notificationSvc NotificationService,
) LoanService {
	return &loanServiceImpl{
		loanRepo:        loanRepo,
		productRepo:     productRepo,
		approvalRepo:    approvalRepo,
		installmentRepo: installmentRepo,
		personnelRepo:   personnelRepo,
		userRepo:        userRepo,
		notifier:        notif,
		notificationSvc: notificationSvc,
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
	// Conditional write: a no-op if the user confirmed in the meantime.
	now := time.Now()
	const reason = "Tidak ada konfirmasi user dalam 24 jam"
	if ok, err := s.loanRepo.ExpireUnconfirmed(ctx, loan.ID, now, reason); err == nil && ok {
		r := reason
		loan.Status = model.LoanStatusCancelled
		loan.RejectionReason = &r
		loan.RejectedAt = &now
	}
}

func (s *loanServiceImpl) GetMyHistory(ctx context.Context, personnelID uuid.UUID, page, limit int, filters map[string]interface{}) ([]model.LoanResponse, int64, error) {
	if filters == nil {
		filters = map[string]interface{}{}
	}
	filters["personnel_id"] = personnelID
	return s.GetAll(ctx, page, limit, filters)
}

func (s *loanServiceImpl) GetMyHistoryByUser(ctx context.Context, userID uuid.UUID, page, limit int, filters map[string]interface{}) (*MyHistoryResult, error) {
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err != nil || personnel == nil {
		return nil, errors.New("personnel not found for current user")
	}
	items, total, err := s.GetMyHistory(ctx, personnel.ID, page, limit, filters)
	if err != nil {
		return nil, err
	}
	return &MyHistoryResult{Items: items, Total: total}, nil
}

func (s *loanServiceImpl) Create(ctx context.Context, req *model.CreateLoanRequest, actorUserID uuid.UUID, actorRole string) (*model.LoanResponse, error) {
	if req.LoanAmount < constants.LoanMinAmount {
		return nil, fmt.Errorf("jumlah pinjaman minimal Rp %.0f", constants.LoanMinAmount)
	}
	if req.LoanAmount > constants.LoanMaxAmount {
		return nil, fmt.Errorf("jumlah pinjaman maksimal Rp %.0f", constants.LoanMaxAmount)
	}
	if req.TenorMonths > constants.LoanMaxTenorMonths {
		return nil, fmt.Errorf("tenor maksimal %d bulan", constants.LoanMaxTenorMonths)
	}
	if req.TermsAccepted == nil || !*req.TermsAccepted {
		return nil, errors.New("Anda harus menyetujui seluruh pernyataan pada Persetujuan & Validasi sebelum mengajukan pinjaman")
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

	if req.SubmitImmediately {
		missingIdentity := []string{}
		if personnel.KtpDocument == nil || *personnel.KtpDocument == "" {
			missingIdentity = append(missingIdentity, "KTP")
		}
		if personnel.NpwpDocument == nil || *personnel.NpwpDocument == "" {
			missingIdentity = append(missingIdentity, "NPWP")
		}
		if len(missingIdentity) > 0 {
			return nil, fmt.Errorf(
				"dokumen %s belum ada di data personel — hubungi admin perusahaan untuk melengkapinya",
				strings.Join(missingIdentity, "/"),
			)
		}
	}

	interestRate := constants.LoanDefaultInterestRatePct
	registrationFee := constants.LoanDefaultRegistrationFee
	provisiRate := constants.LoanDefaultProvisiRatePct
	penaltyEarlyPayoff := constants.LoanDefaultPenaltyEarlyPayoff
	penaltyRunningInterest := constants.LoanDefaultPenaltyRunningInterest
	var productID *uuid.UUID
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
		interestRate = product.InterestRate
		registrationFee = product.RegistrationFee
		provisiRate = product.ProvisiRate
		penaltyEarlyPayoff = product.PenaltyEarlyPayoff
		penaltyRunningInterest = product.PenaltyRunningInterest
		pid := product.ID
		productID = &pid
	}

	monthly, total := calculateLoanDetails(req.LoanAmount, interestRate, req.TenorMonths)
	provisi, netDisbursed := calculateFeeDetails(req.LoanAmount, registrationFee, provisiRate, req.TenorMonths)

	var bujpID *uuid.UUID
	if personnel.BujpID != uuid.Nil {
		bid := personnel.BujpID
		bujpID = &bid
	}

	termsAcceptedAt := time.Now()

	loan := &model.Loan{
		PersonnelID:            personnelID,
		BujpID:                 bujpID,
		LoanProductID:          productID,
		Purpose:                req.Purpose,
		LoanAmount:             req.LoanAmount,
		InterestRate:           interestRate,
		TenorMonths:            req.TenorMonths,
		MonthlyInstallment:     monthly,
		TotalRepayment:         total,
		RegistrationFee:        registrationFee,
		ProvisiRate:            provisiRate,
		Provisi:                provisi,
		PenaltyEarlyPayoff:     penaltyEarlyPayoff,
		PenaltyRunningInterest: penaltyRunningInterest,
		NetDisbursed:           netDisbursed,
		DeductFromPayroll:      derefBoolDefault(req.DeductFromPayroll, true),
		SelfieDocument:         req.SelfieDocument,
		SelfieKtpDocument:      req.SelfieKtpDocument,
		PksDocument:            req.PksDocument,
		CollateralDocument:     req.CollateralDocument,
		PlacementBujpName:      req.PlacementBujpName,
		PlacementAddress:       req.PlacementAddress,
		PlacementDuration:      req.PlacementDuration,
		TermsAcceptedAt:        &termsAcceptedAt,
		Status:                 model.LoanStatusDraft,
		CreatedBy:              &actorUserID,
	}

	if err := s.createWithNumber(ctx, loan); err != nil {
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
	if req.LoanAmount != nil || req.TenorMonths != nil {
		provisi, netDisbursed := calculateFeeDetails(loan.LoanAmount, loan.RegistrationFee, loan.ProvisiRate, loan.TenorMonths)
		loan.Provisi = provisi
		loan.NetDisbursed = netDisbursed
	}
	if req.DeductFromPayroll != nil {
		loan.DeductFromPayroll = *req.DeductFromPayroll
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

	var createdNotifications []model.Notification
	txErr := s.loanRepo.DB().Transaction(func(tx *gorm.DB) error {
		txLoanRepo := s.loanRepo.WithTx(tx)
		txApprovalRepo := s.approvalRepo.WithTx(tx)

		claimed, cerr := txLoanRepo.ClaimSubmission(ctx, loan.ID, now)
		if cerr != nil {
			return cerr
		}
		if !claimed {
			return errors.New("only draft loans can be submitted")
		}
		if cerr := txApprovalRepo.CreateMany(ctx, buildInitialApprovals(loan.ID)); cerr != nil {
			return cerr
		}
		if s.notificationSvc != nil {
			rows, nerr := s.notificationSvc.NotifyLoanSubmitted(ctx, tx, loan)
			if nerr != nil {
				return nerr
			}
			createdNotifications = rows
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	if fresh, ferr := s.loanRepo.FindByID(ctx, loan.ID); ferr == nil && fresh != nil {
		s.notifier.LoanSubmitted(ctx, fresh)
	}
	if s.notificationSvc != nil {
		s.notificationSvc.PushRealtime(createdNotifications)
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
	var createdNotifications []model.Notification
	txErr := s.loanRepo.DB().Transaction(func(tx *gorm.DB) error {
		txLoanRepo := s.loanRepo.WithTx(tx)
		// Re-check atomically inside the write itself — the plain read
		// above can be stale by the time we get here (e.g. an approval or
		// disbursement already moved the loan past the point of no
		// return), and an unconditional Update() would silently clobber
		// that concurrent progress.
		claimed, cerr := txLoanRepo.ClaimCancel(ctx, loan.ID)
		if cerr != nil {
			return cerr
		}
		if !claimed {
			return errors.New("loan cannot be cancelled in current status")
		}
		if cerr := txLoanRepo.UpdateColumns(ctx, loan.ID, map[string]interface{}{"rejection_reason": reason}); cerr != nil {
			return cerr
		}
		loan.Status = model.LoanStatusCancelled
		loan.RejectionReason = reason
		if s.notificationSvc != nil {
			rows, nerr := s.notificationSvc.NotifyLoanCancelled(ctx, tx, loan)
			if nerr != nil {
				return nerr
			}
			createdNotifications = rows
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	if s.notificationSvc != nil {
		s.notificationSvc.PushRealtime(createdNotifications)
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) UserConfirm(ctx context.Context, id uuid.UUID, action string, reason *string) (*model.LoanResponse, error) {
	switch action {
	case "accept", "accepted":
		action = "accept"
	case "decline", "declined", "reject", "rejected":
		action = "decline"
	default:
		return nil, errors.New("invalid action: must be 'accept' or 'decline'")
	}

	loan, err := s.loanRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("loan not found")
	}
	if loan.Status != model.LoanStatusPendingUserConfirmation {
		return nil, errors.New("loan is not pending user confirmation")
	}
	now := time.Now()
	expired := loan.UserConfirmationDeadline != nil && now.After(*loan.UserConfirmationDeadline)

	if expired {
		_, _ = s.loanRepo.ExpireUnconfirmed(ctx, loan.ID, now, "User confirmation deadline expired")
		return nil, errors.New("user confirmation deadline expired; loan cancelled")
	}

	claimed, err := s.loanRepo.ClaimUserConfirmation(ctx, loan.ID, now)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, errors.New("loan is not pending user confirmation")
	}

	if action == "decline" {
		if err := s.loanRepo.UpdateColumns(ctx, loan.ID, map[string]interface{}{
			"status":           model.LoanStatusCancelled,
			"rejection_reason": reason,
		}); err != nil {
			return nil, err
		}
	} else if err := s.disburse(ctx, loan, nil); err != nil {
		return nil, err
	}
	if fresh, ferr := s.loanRepo.FindByID(ctx, loan.ID); ferr == nil && fresh != nil {
		s.notifier.LoanUserConfirmed(ctx, fresh, action == "accept", reason)
	}
	return s.GetByID(ctx, loan.ID)
}

func (s *loanServiceImpl) Statistics(ctx context.Context) (map[string]interface{}, error) {
	return s.statusCounts(ctx, uuid.Nil, uuid.Nil)
}

// statusCounts returns one entry per known status (zero when absent) from a
// single grouped query.
func (s *loanServiceImpl) statusCounts(ctx context.Context, bujpID, personnelID uuid.UUID) (map[string]interface{}, error) {
	counts, err := s.loanRepo.CountsByStatus(ctx, bujpID, personnelID)
	if err != nil {
		return nil, err
	}
	stats := emptyLoanStats()
	for _, st := range loanStatStatuses() {
		stats[st] = counts[st]
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
	case "super_admin", "admin", "bprks":
		// no scoping
	default:
		// Unknown role: be safe — return zeros.
		return emptyLoanStats(), nil
	}

	return s.statusCounts(ctx, bujpID, personnelID)
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

func (s *loanServiceImpl) resolvePersonnelID(ctx context.Context, requested *uuid.UUID, actorUserID uuid.UUID) (uuid.UUID, error) {
	if requested != nil && *requested != uuid.Nil {
		return *requested, nil
	}
	user, err := s.userRepo.FindByID(ctx, actorUserID)
	if err != nil {
		return uuid.Nil, errors.New("authenticated user not found")
	}
	personnel, err := s.personnelRepo.FindByUserID(ctx, user.ID)
	if err != nil || personnel == nil {
		return uuid.Nil, errors.New("personnel_id is required")
	}
	return personnel.ID, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// createWithNumber inserts the loan with the next LN-YYYY-MM-NNNN number.
// The number is derived from a count, so two simultaneous requests can pick
// the same one; the unique index rejects the loser, which simply retries with
// the next number.
func (s *loanServiceImpl) createWithNumber(ctx context.Context, loan *model.Loan) error {
	count, err := s.loanRepo.CountThisMonth(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for attempt := int64(1); ; attempt++ {
		loan.LoanNumber = fmt.Sprintf("LN-%04d-%02d-%04d", now.Year(), now.Month(), count+attempt)
		err = s.loanRepo.Create(ctx, loan)
		if err == nil || attempt >= 5 || !isUniqueViolation(err) {
			return err
		}
	}
}

// buildInitialApprovals is a pure builder (no repo call) so it can be used
// both outside a transaction (none left to, but kept generic) and — as it is
// now — inside Submit's db.Transaction via a tx-bound repository.
func buildInitialApprovals(loanID uuid.UUID) []model.LoanApproval {
	return []model.LoanApproval{
		{
			LoanID:            loanID,
			ApprovalLevel:     model.LoanApprovalLevelBujp,
			ApprovalLevelName: "Admin Perusahaan",
			Status:            model.LoanApprovalStatusPending,
		},
		{
			LoanID:            loanID,
			ApprovalLevel:     model.LoanApprovalLevelPusat,
			ApprovalLevelName: "Admin Pusat",
			Status:            model.LoanApprovalStatusPending,
		},
		{
			LoanID:            loanID,
			ApprovalLevel:     model.LoanApprovalLevelBprks,
			ApprovalLevelName: "BPRKS",
			Status:            model.LoanApprovalStatusPending,
		},
	}
}

// disburse moves an approved loan to `active` and creates its installment
// schedule in one transaction, so a failure can never leave an active loan
// without installments. Only the caller that wins the status claim proceeds,
// which stops a doubled click, an admin "Cairkan" and the applicant's own
// accept from creating the schedule twice.
//
// req carries the admin's "Catat Pencairan Dana" form; it is nil when the
// applicant accepts, in which case dates default to now / the 1st of next month.
func (s *loanServiceImpl) disburse(ctx context.Context, loan *model.Loan, req *model.DisburseLoanRequest) error {
	return s.loanRepo.DB().Transaction(func(tx *gorm.DB) error {
		txLoanRepo := s.loanRepo.WithTx(tx)
		claimed, err := txLoanRepo.ClaimDisbursement(ctx, loan.ID)
		if err != nil {
			return err
		}
		if !claimed {
			return errors.New("loan is already being disbursed or has already been disbursed")
		}
		installments := s.applyDisbursement(loan, req)
		if err := s.installmentRepo.WithTx(tx).CreateMany(ctx, installments); err != nil {
			return err
		}
		return txLoanRepo.UpdateColumns(ctx, loan.ID, map[string]interface{}{
			"disbursement_date":      loan.DisbursementDate,
			"first_installment_date": loan.FirstInstallmentDate,
			"disbursement_method":    loan.DisbursementMethod,
			"disbursement_account":   loan.DisbursementAccount,
		})
	})
}

// applyDisbursement fills the disbursement fields on loan and returns the
// installment schedule. The last installment absorbs rounding so the schedule
// always adds up to exactly principal + interest.
func (s *loanServiceImpl) applyDisbursement(loan *model.Loan, req *model.DisburseLoanRequest) []model.LoanInstallment {
	now := time.Now()
	if req != nil && req.DisbursementDate != nil && !req.DisbursementDate.IsZero() {
		d := req.DisbursementDate.Time
		loan.DisbursementDate = &d
	} else {
		loan.DisbursementDate = &now
	}

	if req != nil && req.FirstInstallmentDate != nil && !req.FirstInstallmentDate.IsZero() {
		f := req.FirstInstallmentDate.Time
		loan.FirstInstallmentDate = &f
	} else {
		first := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local)
		loan.FirstInstallmentDate = &first
	}

	if req != nil && req.DisbursementMethod != nil && *req.DisbursementMethod != "" {
		loan.DisbursementMethod = req.DisbursementMethod
	}
	if req != nil && req.DisbursementAccount != nil && *req.DisbursementAccount != "" {
		loan.DisbursementAccount = req.DisbursementAccount
	}

	approvedAmount := loan.LoanAmount
	if loan.ApprovedAmount != nil {
		approvedAmount = *loan.ApprovedAmount
	}
	approvedTenor := loan.TenorMonths
	if loan.ApprovedTenor != nil {
		approvedTenor = *loan.ApprovedTenor
	}

	totalInterest := round2(approvedAmount * (loan.InterestRate / 100) * float64(approvedTenor))
	monthlyPrincipal := round2(approvedAmount / float64(approvedTenor))
	monthlyInterest := round2(totalInterest / float64(approvedTenor))

	installments := make([]model.LoanInstallment, 0, approvedTenor)
	var paidPrincipal, paidInterest float64
	for i := 0; i < approvedTenor; i++ {
		principal, interest := monthlyPrincipal, monthlyInterest
		if i == approvedTenor-1 {
			principal = round2(approvedAmount - paidPrincipal)
			interest = round2(totalInterest - paidInterest)
		}
		paidPrincipal += principal
		paidInterest += interest
		amt := round2(principal + interest)
		installments = append(installments, model.LoanInstallment{
			LoanID:            loan.ID,
			InstallmentNumber: i + 1,
			DueDate:           loan.FirstInstallmentDate.AddDate(0, i, 0),
			PrincipalAmount:   principal,
			InterestAmount:    interest,
			InstallmentAmount: amt,
			RemainingAmount:   amt,
			Status:            model.LoanInstallmentStatusPending,
		})
	}
	loan.Status = model.LoanStatusActive
	return installments
}

func calculateLoanDetails(principal, monthlyRatePct float64, tenorMonths int) (monthly, total float64) {
	if tenorMonths <= 0 {
		return 0, 0
	}
	totalInterest := principal * (monthlyRatePct / 100) * float64(tenorMonths)
	total = round2(principal + totalInterest)
	monthly = round2(total / float64(tenorMonths))
	return
}

func calculateFeeDetails(principal, registrationFee, provisiRatePct float64, tenorMonths int) (provisi, netDisbursed float64) {
	provisi = round2(principal * (provisiRatePct / 100) * float64(tenorMonths))
	netDisbursed = round2(principal - registrationFee - provisi)
	return
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func resolveLoanIdentityDoc(loanDoc *string, personnel *model.Personnel, get func(*model.Personnel) *string) *string {
	if loanDoc != nil && *loanDoc != "" {
		return loanDoc
	}
	if personnel == nil {
		return nil
	}
	return get(personnel)
}

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
		ID:                     l.ID,
		LoanNumber:             l.LoanNumber,
		PersonnelID:            l.PersonnelID,
		BujpID:                 l.BujpID,
		LoanProductID:          l.LoanProductID,
		Purpose:                l.Purpose,
		LoanAmount:             l.LoanAmount,
		InterestRate:           l.InterestRate,
		TenorMonths:            l.TenorMonths,
		MonthlyInstallment:     l.MonthlyInstallment,
		TotalRepayment:         l.TotalRepayment,
		RegistrationFee:        l.RegistrationFee,
		ProvisiRate:            l.ProvisiRate,
		Provisi:                l.Provisi,
		PenaltyEarlyPayoff:     l.PenaltyEarlyPayoff,
		PenaltyRunningInterest: l.PenaltyRunningInterest,
		NetDisbursed:           l.NetDisbursed,
		ApprovedAmount:         l.ApprovedAmount,
		ApprovedTenor:          l.ApprovedTenor,
		DisbursementDate:       dateStrPtr(l.DisbursementDate),
		FirstInstallmentDate:   dateStrPtr(l.FirstInstallmentDate),
		DisbursementMethod:     l.DisbursementMethod,
		DisbursementAccount:    l.DisbursementAccount,
		DeductFromPayroll:      l.DeductFromPayroll,
		// Prefer the loan's own KTP/NPWP if present (legacy loans created
		// before these moved to Personnel); otherwise resolve from
		// Personnel, where they're now uploaded once instead of per loan.
		KtpDocument:              resolveLoanIdentityDoc(l.KtpDocument, l.Personnel, func(p *model.Personnel) *string { return p.KtpDocument }),
		NpwpDocument:             resolveLoanIdentityDoc(l.NpwpDocument, l.Personnel, func(p *model.Personnel) *string { return p.NpwpDocument }),
		SelfieDocument:           l.SelfieDocument,
		SelfieKtpDocument:        l.SelfieKtpDocument,
		PksDocument:              l.PksDocument,
		CollateralDocument:       l.CollateralDocument,
		PlacementCompanyName:     l.PlacementCompanyName,
		PlacementBujpName:        l.PlacementBujpName,
		PlacementAddress:         l.PlacementAddress,
		PlacementDuration:        l.PlacementDuration,
		PlacementLocation:        l.PlacementLocation,
		TermsAcceptedAt:          l.TermsAcceptedAt,
		Status:                   l.Status,
		SubmittedAt:              l.SubmittedAt,
		ApprovedAt:               l.ApprovedAt,
		ApprovedPusatAt:          l.ApprovedPusatAt,
		ApprovedBprksAt:          l.ApprovedBprksAt,
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
	}
	if finalAmount != l.LoanAmount || finalTenor != l.TenorMonths {
		provisi, netDisbursed := calculateFeeDetails(finalAmount, l.RegistrationFee, l.ProvisiRate, finalTenor)
		r.Provisi = provisi
		r.NetDisbursed = netDisbursed
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
