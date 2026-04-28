package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PayrollRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Payroll, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Payroll, error)
	FindByPersonnelAndPeriod(ctx context.Context, personnelID uuid.UUID, period string) (*model.Payroll, error)
	Create(ctx context.Context, payroll *model.Payroll) error
	CreateWithDetails(ctx context.Context, payroll *model.Payroll, details []model.PayrollDetail) error
	Update(ctx context.Context, payroll *model.Payroll) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type payrollRepositoryImpl struct {
	db *gorm.DB
}

func NewPayrollRepository(db *gorm.DB) PayrollRepository {
	return &payrollRepositoryImpl{db: db}
}

func (r *payrollRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Payroll, int64, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var payrolls []model.Payroll
	var total int64

	query := r.db.WithContext(ctxTimeout).Model(&model.Payroll{})

	// Apply filters
	if personnelID, ok := filters["personnel_id"].(string); ok && personnelID != "" {
		query = query.Where("personnel_id = ?", personnelID)
	}
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Payroll{}, 0, nil
	}
	if bujpUUID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpUUID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpUUID)
	} else if bujpID, ok := filters["bujp_id"].(string); ok && bujpID != "" {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if period, ok := filters["period"].(string); ok && period != "" {
		query = query.Where("period = ?", period)
	}
	if status, ok := filters["payment_status"].(string); ok && status != "" {
		query = query.Where("payment_status = ?", status)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get data with pagination and preload
	offset := (page - 1) * limit
	if err := query.
		Preload("Personnel").
		Preload("Bujp").
		Preload("Creator").
		Preload("PayrollDetails.SalaryComponent").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&payrolls).Error; err != nil {
		return nil, 0, err
	}

	return payrolls, total, nil
}

func (r *payrollRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Payroll, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var payroll model.Payroll
	if err := r.db.WithContext(ctxTimeout).
		Preload("Personnel").
		Preload("Bujp").
		Preload("Creator").
		Preload("PayrollDetails.SalaryComponent").
		First(&payroll, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &payroll, nil
}

func (r *payrollRepositoryImpl) FindByPersonnelAndPeriod(ctx context.Context, personnelID uuid.UUID, period string) (*model.Payroll, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var payroll model.Payroll
	if err := r.db.WithContext(ctxTimeout).
		Where("personnel_id = ? AND period = ?", personnelID, period).
		First(&payroll).Error; err != nil {
		return nil, err
	}

	return &payroll, nil
}

func (r *payrollRepositoryImpl) Create(ctx context.Context, payroll *model.Payroll) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Create(payroll).Error
}

func (r *payrollRepositoryImpl) CreateWithDetails(ctx context.Context, payroll *model.Payroll, details []model.PayrollDetail) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Transaction(func(tx *gorm.DB) error {
		// Create payroll
		if err := tx.Create(payroll).Error; err != nil {
			return fmt.Errorf("failed to create payroll: %w", err)
		}

		// Create details
		if len(details) > 0 {
			for i := range details {
				details[i].PayrollID = payroll.ID
			}
			if err := tx.Create(&details).Error; err != nil {
				return fmt.Errorf("failed to create payroll details: %w", err)
			}
		}

		return nil
	})
}

func (r *payrollRepositoryImpl) Update(ctx context.Context, payroll *model.Payroll) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Save(payroll).Error
}

func (r *payrollRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Transaction(func(tx *gorm.DB) error {
		// Delete details first (cascade)
		if err := tx.Where("payroll_id = ?", id).Delete(&model.PayrollDetail{}).Error; err != nil {
			return err
		}
		// Delete payroll
		return tx.Delete(&model.Payroll{}, "id = ?", id).Error
	})
}
