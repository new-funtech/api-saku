package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShiftRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Shift, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Shift, error)
	FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Shift, error)
	Create(ctx context.Context, shift *model.Shift) error
	Update(ctx context.Context, shift *model.Shift) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type shiftRepositoryImpl struct {
	db *gorm.DB
}

func NewShiftRepository(db *gorm.DB) ShiftRepository {
	return &shiftRepositoryImpl{db: db}
}

func (r *shiftRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *shiftRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Shift, int64, error) {
	var shifts []model.Shift
	var total int64

	query := r.withContext(ctx).Model(&model.Shift{})

	// Apply filters
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Shift{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	// Preload bujp relation
	query = query.Preload("Bujp", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "code", "name")
	})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&shifts).Error
	return shifts, total, err
}

func (r *shiftRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Shift, error) {
	var shift model.Shift
	if err := r.withContext(ctx).Preload("Bujp").Where("id = ?", id).First(&shift).Error; err != nil {
		return nil, err
	}
	return &shift, nil
}

func (r *shiftRepositoryImpl) FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Shift, error) {
	var shifts []model.Shift
	err := r.withContext(ctx).Where("bujp_id = ?", bujpID).Find(&shifts).Error
	return shifts, err
}

func (r *shiftRepositoryImpl) Create(ctx context.Context, shift *model.Shift) error {
	return r.withContext(ctx).Create(shift).Error
}

func (r *shiftRepositoryImpl) Update(ctx context.Context, shift *model.Shift) error {
	// Use Updates() instead of Save() for better performance
	return r.withContext(ctx).Model(shift).Updates(shift).Error
}

func (r *shiftRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Shift{}).Error
}
