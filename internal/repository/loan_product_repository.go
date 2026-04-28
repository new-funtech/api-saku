package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanProductRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanProduct, int64, error)
	FindActive(ctx context.Context, bujpID *uuid.UUID) ([]model.LoanProduct, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.LoanProduct, error)
	Create(ctx context.Context, lp *model.LoanProduct) error
	Update(ctx context.Context, lp *model.LoanProduct) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountThisMonth(ctx context.Context) (int64, error)
}

type loanProductRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanProductRepository(db *gorm.DB) LoanProductRepository {
	return &loanProductRepositoryImpl{db: db}
}

func (r *loanProductRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanProduct, int64, error) {
	var items []model.LoanProduct
	var total int64

	q := r.db.WithContext(ctx).Model(&model.LoanProduct{}).Preload("Bujp")
	if v, ok := filters["bujp_id"].(uuid.UUID); ok && v != uuid.Nil {
		q = q.Where("bujp_id = ? OR bujp_id IS NULL", v)
	}
	if v, ok := filters["is_active"].(*bool); ok && v != nil {
		q = q.Where("is_active = ?", *v)
	}
	if v, ok := filters["search"].(string); ok && v != "" {
		q = q.Where("name ILIKE ? OR code ILIKE ?", "%"+v+"%", "%"+v+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *loanProductRepositoryImpl) FindActive(ctx context.Context, bujpID *uuid.UUID) ([]model.LoanProduct, error) {
	var items []model.LoanProduct
	q := r.db.WithContext(ctx).Where("is_active = ?", true)
	if bujpID != nil {
		q = q.Where("bujp_id IS NULL OR bujp_id = ?", *bujpID)
	}
	err := q.Order("name ASC").Find(&items).Error
	return items, err
}

func (r *loanProductRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.LoanProduct, error) {
	var lp model.LoanProduct
	if err := r.db.WithContext(ctx).Preload("Bujp").First(&lp, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &lp, nil
}

func (r *loanProductRepositoryImpl) Create(ctx context.Context, lp *model.LoanProduct) error {
	return r.db.WithContext(ctx).Create(lp).Error
}

func (r *loanProductRepositoryImpl) Update(ctx context.Context, lp *model.LoanProduct) error {
	return r.db.WithContext(ctx).Save(lp).Error
}

func (r *loanProductRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.LoanProduct{}, "id = ?", id).Error
}

func (r *loanProductRepositoryImpl) CountThisMonth(ctx context.Context) (int64, error) {
	var c int64
	err := r.db.WithContext(ctx).Model(&model.LoanProduct{}).
		Where("DATE_TRUNC('month', created_at) = DATE_TRUNC('month', CURRENT_DATE)").
		Count(&c).Error
	return c, err
}
