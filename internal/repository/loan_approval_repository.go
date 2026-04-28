package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanApprovalRepository interface {
	FindByLoanID(ctx context.Context, loanID uuid.UUID) ([]model.LoanApproval, error)
	FindCurrentPendingForLoan(ctx context.Context, loanID uuid.UUID) (*model.LoanApproval, error)
	FindPendingForRole(ctx context.Context, role string, bujpID *uuid.UUID, page, limit int) ([]model.LoanApproval, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.LoanApproval, error)
	Create(ctx context.Context, a *model.LoanApproval) error
	CreateMany(ctx context.Context, items []model.LoanApproval) error
	Update(ctx context.Context, a *model.LoanApproval) error
}

type loanApprovalRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanApprovalRepository(db *gorm.DB) LoanApprovalRepository {
	return &loanApprovalRepositoryImpl{db: db}
}

func (r *loanApprovalRepositoryImpl) FindByLoanID(ctx context.Context, loanID uuid.UUID) ([]model.LoanApproval, error) {
	var items []model.LoanApproval
	err := r.db.WithContext(ctx).Preload("Approver").
		Where("loan_id = ?", loanID).
		Order("approval_level ASC").
		Find(&items).Error
	return items, err
}

func (r *loanApprovalRepositoryImpl) FindCurrentPendingForLoan(ctx context.Context, loanID uuid.UUID) (*model.LoanApproval, error) {
	var a model.LoanApproval
	err := r.db.WithContext(ctx).
		Where("loan_id = ? AND status = ?", loanID, model.LoanApprovalStatusPending).
		Order("approval_level ASC").
		First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *loanApprovalRepositoryImpl) FindPendingForRole(ctx context.Context, role string, bujpID *uuid.UUID, page, limit int) ([]model.LoanApproval, int64, error) {
	var items []model.LoanApproval
	var total int64

	level := model.LoanApprovalLevelBujp
	if role == "admin" || role == "super_admin" {
		level = model.LoanApprovalLevelPusat
	}

	q := r.db.WithContext(ctx).Model(&model.LoanApproval{}).
		Preload("Loan.Personnel").
		Preload("Loan.Bujp").
		Preload("Loan.LoanProduct").
		Where("loan_approvals.approval_level = ? AND loan_approvals.status = ?", level, model.LoanApprovalStatusPending)

	if level == model.LoanApprovalLevelBujp && bujpID != nil {
		q = q.Joins("JOIN loans ON loans.id = loan_approvals.loan_id").
			Where("loans.bujp_id = ?", *bujpID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("loan_approvals.created_at ASC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *loanApprovalRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.LoanApproval, error) {
	var a model.LoanApproval
	// Preload nested Loan relations to avoid N+1 when callers access
	// approval.Loan.Personnel / approval.Loan.Bujp / approval.Loan.LoanProduct.
	if err := r.db.WithContext(ctx).
		Preload("Loan.Personnel").
		Preload("Loan.Bujp").
		Preload("Loan.LoanProduct").
		Preload("Approver").
		First(&a, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *loanApprovalRepositoryImpl) Create(ctx context.Context, a *model.LoanApproval) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *loanApprovalRepositoryImpl) CreateMany(ctx context.Context, items []model.LoanApproval) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *loanApprovalRepositoryImpl) Update(ctx context.Context, a *model.LoanApproval) error {
	return r.db.WithContext(ctx).Save(a).Error
}
