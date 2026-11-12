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
	WithTx(tx *gorm.DB) LoanInstallmentRepository
	CreateMany(ctx context.Context, items []model.LoanInstallment) error
	Update(ctx context.Context, item *model.LoanInstallment) error
	DueStats(ctx context.Context, bujpID uuid.UUID, today time.Time, horizonDays int) (overdue int64, overdueAmount float64, dueSoon int64, err error)
	FindUnpaidByPersonnel(ctx context.Context, personnelID uuid.UUID) ([]model.LoanInstallment, error)
	MarkOverdueBefore(ctx context.Context, before time.Time) (int64, error)
	GetDueForPayroll(ctx context.Context, personnelID uuid.UUID, period time.Time) ([]model.LoanInstallment, error)
}

type loanInstallmentRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanInstallmentRepository(db *gorm.DB) LoanInstallmentRepository {
	return &loanInstallmentRepositoryImpl{db: db}
}

func (r *loanInstallmentRepositoryImpl) WithTx(tx *gorm.DB) LoanInstallmentRepository {
	return &loanInstallmentRepositoryImpl{db: tx}
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

// FindUnpaidByPersonnel returns the unpaid installments of the person's running loans, soonest due first.
func (r *loanInstallmentRepositoryImpl) FindUnpaidByPersonnel(ctx context.Context, personnelID uuid.UUID) ([]model.LoanInstallment, error) {
	var items []model.LoanInstallment
	err := r.db.WithContext(ctx).
		Joins("JOIN loans ON loans.id = loan_installments.loan_id").
		Where("loans.personnel_id = ? AND loans.status IN ? AND loan_installments.status IN ?",
			personnelID,
			[]string{model.LoanStatusActive, model.LoanStatusDisbursed},
			[]string{model.LoanInstallmentStatusPending, model.LoanInstallmentStatusOverdue}).
		Preload("Loan").
		Order("loan_installments.due_date ASC, loan_installments.installment_number ASC").
		Find(&items).Error
	return items, err
}

// DueStats counts overdue and soon-due installments of running loans; a nil bujpID means everyone.
func (r *loanInstallmentRepositoryImpl) DueStats(ctx context.Context, bujpID uuid.UUID, today time.Time, horizonDays int) (overdue int64, overdueAmount float64, dueSoon int64, err error) {
	base := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&model.LoanInstallment{}).
			Joins("JOIN loans ON loans.id = loan_installments.loan_id").
			Where("loans.status IN ? AND loan_installments.status IN ?",
				[]string{model.LoanStatusActive, model.LoanStatusDisbursed},
				[]string{model.LoanInstallmentStatusPending, model.LoanInstallmentStatusOverdue})
		if bujpID != uuid.Nil {
			q = q.Where("loans.bujp_id = ?", bujpID)
		}
		return q
	}
	var late struct {
		Count  int64
		Amount float64
	}
	if err = base().Where("loan_installments.due_date < ?", today).
		Select("COUNT(*) AS count, COALESCE(SUM(loan_installments.installment_amount + loan_installments.late_fee - loan_installments.paid_amount), 0) AS amount").
		Scan(&late).Error; err != nil {
		return
	}
	overdue, overdueAmount = late.Count, late.Amount
	err = base().Where("loan_installments.due_date >= ? AND loan_installments.due_date <= ?", today, today.AddDate(0, 0, horizonDays)).
		Count(&dueSoon).Error
	return
}
