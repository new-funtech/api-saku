package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanInstallmentRepository interface {
	FindByLoanID(ctx context.Context, loanID uuid.UUID) ([]model.LoanInstallment, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.LoanInstallment, error)
	CreateMany(ctx context.Context, items []model.LoanInstallment) error
	Update(ctx context.Context, item *model.LoanInstallment) error
	MarkOverdueBefore(ctx context.Context, before time.Time) (int64, error)
	GetDueForPayroll(ctx context.Context, personnelID uuid.UUID, period time.Time) ([]model.LoanInstallment, error)
}

type loanInstallmentRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanInstallmentRepository(db *gorm.DB) LoanInstallmentRepository {
	return &loanInstallmentRepositoryImpl{db: db}
}

func (r *loanInstallmentRepositoryImpl) FindByLoanID(ctx context.Context, loanID uuid.UUID) ([]model.LoanInstallment, error) {
	var items []model.LoanInstallment
	err := r.db.WithContext(ctx).Where("loan_id = ?", loanID).Order("installment_number ASC").Find(&items).Error
	return items, err
}

func (r *loanInstallmentRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.LoanInstallment, error) {
	var i model.LoanInstallment
	if err := r.db.WithContext(ctx).First(&i, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *loanInstallmentRepositoryImpl) CreateMany(ctx context.Context, items []model.LoanInstallment) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *loanInstallmentRepositoryImpl) Update(ctx context.Context, item *model.LoanInstallment) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *loanInstallmentRepositoryImpl) MarkOverdueBefore(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.LoanInstallment{}).
		Where("status = ? AND due_date < ?", model.LoanInstallmentStatusPending, before).
		Update("status", model.LoanInstallmentStatusOverdue)
	return res.RowsAffected, res.Error
}

func (r *loanInstallmentRepositoryImpl) GetDueForPayroll(ctx context.Context, personnelID uuid.UUID, period time.Time) ([]model.LoanInstallment, error) {
	var items []model.LoanInstallment
	first := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1)
	err := r.db.WithContext(ctx).
		Joins("JOIN loans ON loans.id = loan_installments.loan_id").
		Where("loans.personnel_id = ?", personnelID).
		Where("loans.deduct_from_payroll = ?", true).
		Where("loans.status IN ?", []string{model.LoanStatusActive, model.LoanStatusDisbursed}).
		Where("loan_installments.status IN ?", []string{model.LoanInstallmentStatusPending, model.LoanInstallmentStatusOverdue}).
		Where("loan_installments.due_date <= ?", last).
		Where("loan_installments.due_date >= ?", first.AddDate(0, -6, 0)).
		Order("loan_installments.due_date ASC").
		Find(&items).Error
	return items, err
}
