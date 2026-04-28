package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SalaryComponentRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.SalaryComponent, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.SalaryComponent, error)
	FindByCode(ctx context.Context, code string) (*model.SalaryComponent, error)
	Create(ctx context.Context, component *model.SalaryComponent) error
	Update(ctx context.Context, component *model.SalaryComponent) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type salaryComponentRepositoryImpl struct {
	db *gorm.DB
}

func NewSalaryComponentRepository(db *gorm.DB) SalaryComponentRepository {
	return &salaryComponentRepositoryImpl{db: db}
}

func (r *salaryComponentRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.SalaryComponent, int64, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var components []model.SalaryComponent
	var total int64

	query := r.db.WithContext(ctxTimeout).Model(&model.SalaryComponent{})

	// Apply filters
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.SalaryComponent{}, 0, nil
	}
	if bujpUUID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpUUID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpUUID)
	} else if bujpID, ok := filters["bujp_id"].(string); ok && bujpID != "" {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if componentType, ok := filters["type"].(string); ok && componentType != "" {
		query = query.Where("type = ?", componentType)
	}
	if isActive, ok := filters["is_active"].(bool); ok {
		query = query.Where("is_active = ?", isActive)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get data with pagination and preload
	offset := (page - 1) * limit
	if err := query.Preload("Bujp").Offset(offset).Limit(limit).Find(&components).Error; err != nil {
		return nil, 0, err
	}

	return components, total, nil
}

func (r *salaryComponentRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.SalaryComponent, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var component model.SalaryComponent
	if err := r.db.WithContext(ctxTimeout).Preload("Bujp").First(&component, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &component, nil
}

func (r *salaryComponentRepositoryImpl) FindByCode(ctx context.Context, code string) (*model.SalaryComponent, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var component model.SalaryComponent
	if err := r.db.WithContext(ctxTimeout).Where("code = ?", code).First(&component).Error; err != nil {
		return nil, err
	}

	return &component, nil
}

func (r *salaryComponentRepositoryImpl) Create(ctx context.Context, component *model.SalaryComponent) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Create(component).Error
}

func (r *salaryComponentRepositoryImpl) Update(ctx context.Context, component *model.SalaryComponent) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Save(component).Error
}

func (r *salaryComponentRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Delete(&model.SalaryComponent{}, "id = ?", id).Error
}
