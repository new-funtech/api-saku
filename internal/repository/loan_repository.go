package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Loan, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Loan, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Loan, int64, error)
	Create(ctx context.Context, l *model.Loan) error
	Update(ctx context.Context, l *model.Loan) error
	UpdateColumns(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	ClaimUserConfirmation(ctx context.Context, id uuid.UUID, confirmedAt time.Time) (bool, error)
	ClaimSubmission(ctx context.Context, id uuid.UUID, submittedAt time.Time) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountThisMonth(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	CountByStatusScoped(ctx context.Context, status string, bujpID, personnelID uuid.UUID) (int64, error)
	GetActiveByPersonnel(ctx context.Context, personnelID uuid.UUID) ([]model.Loan, error)
	CountByPersonnelID(ctx context.Context, personnelID uuid.UUID) (int64, error)
}

type loanRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanRepository(db *gorm.DB) LoanRepository {
	return &loanRepositoryImpl{db: db}
}

func (r *loanRepositoryImpl) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Loan{}).
		Preload("Personnel").
		Preload("Bujp").
		Preload("LoanProduct").
		Preload("Approvals.Approver").
		Preload("Installments", func(db *gorm.DB) *gorm.DB {
			return db.Order("installment_number ASC")
		})
}

func (r *loanRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Loan, int64, error) {
	var loans []model.Loan
	var total int64

	q := r.baseQuery(ctx)
	if v, ok := filters["force_empty"].(bool); ok && v {
		// Caller requested an empty result set (e.g. unscoped admin).
		return []model.Loan{}, 0, nil
	}
	if v, ok := filters["status"].(string); ok && v != "" {
		q = q.Where("status = ?", v)
	}
	if v, ok := filters["status_in"].([]string); ok && len(v) > 0 {
		q = q.Where("status IN ?", v)
	}
	if v, ok := filters["bujp_id"].(uuid.UUID); ok && v != uuid.Nil {
		q = q.Where("bujp_id = ?", v)
	}
	if v, ok := filters["personnel_id"].(uuid.UUID); ok && v != uuid.Nil {
		q = q.Where("personnel_id = ?", v)
	}
	if v, ok := filters["loan_product_id"].(uuid.UUID); ok && v != uuid.Nil {
		q = q.Where("loan_product_id = ?", v)
	}
	if v, ok := filters["search"].(string); ok && v != "" {
		q = q.Where("loan_number ILIKE ?", "%"+v+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&loans).Error
	return loans, total, err
}

func (r *loanRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Loan, error) {
	var l model.Loan
	if err := r.baseQuery(ctx).
		First(&l, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *loanRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Loan, int64, error) {
	var loans []model.Loan
	var total int64
	q := r.baseQuery(ctx).Where("personnel_id = ?", personnelID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&loans).Error
	return loans, total, err
}

func (r *loanRepositoryImpl) Create(ctx context.Context, l *model.Loan) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *loanRepositoryImpl) Update(ctx context.Context, l *model.Loan) error {
	return r.db.WithContext(ctx).Save(l).Error
}

func (r *loanRepositoryImpl) UpdateColumns(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Loan{}).Where("id = ?", id).Updates(updates).Error
}

func (r *loanRepositoryImpl) ClaimUserConfirmation(ctx context.Context, id uuid.UUID, confirmedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.Loan{}).
		Where("id = ? AND status = ? AND user_confirmed_at IS NULL", id, model.LoanStatusPendingUserConfirmation).
		Update("user_confirmed_at", confirmedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *loanRepositoryImpl) ClaimSubmission(ctx context.Context, id uuid.UUID, submittedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.Loan{}).
		Where("id = ? AND status = ?", id, model.LoanStatusDraft).
		Updates(map[string]interface{}{
			"status":       model.LoanStatusSubmitted,
			"submitted_at": submittedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *loanRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Loan{}, "id = ?", id).Error
}

func (r *loanRepositoryImpl) CountThisMonth(ctx context.Context) (int64, error) {
	var c int64
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	err := r.db.WithContext(ctx).Model(&model.Loan{}).
		Where("created_at >= ?", start).Count(&c).Error
	return c, err
}

func (r *loanRepositoryImpl) CountByStatus(ctx context.Context, status string) (int64, error) {
	var c int64
	err := r.db.WithContext(ctx).Model(&model.Loan{}).Where("status = ?", status).Count(&c).Error
	return c, err
}

func (r *loanRepositoryImpl) CountByStatusScoped(ctx context.Context, status string, bujpID, personnelID uuid.UUID) (int64, error) {
	var c int64
	q := r.db.WithContext(ctx).Model(&model.Loan{}).Where("status = ?", status)
	if bujpID != uuid.Nil {
		q = q.Where("bujp_id = ?", bujpID)
	}
	if personnelID != uuid.Nil {
		q = q.Where("personnel_id = ?", personnelID)
	}
	err := q.Count(&c).Error
	return c, err
}

func (r *loanRepositoryImpl) GetActiveByPersonnel(ctx context.Context, personnelID uuid.UUID) ([]model.Loan, error) {
	var loans []model.Loan
	err := r.db.WithContext(ctx).Where("personnel_id = ? AND status IN ?", personnelID, []string{
		model.LoanStatusActive, model.LoanStatusDisbursed,
	}).Find(&loans).Error
	return loans, err
}

func (r *loanRepositoryImpl) CountByPersonnelID(ctx context.Context, personnelID uuid.UUID) (int64, error) {
	var c int64
	err := r.db.WithContext(ctx).Model(&model.Loan{}).
		Where("personnel_id = ?", personnelID).Count(&c).Error
	return c, err
}
