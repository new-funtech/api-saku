package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LocationRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Location, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Location, error)
	FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Location, error)
	FindByCode(ctx context.Context, code string) (*model.Location, error)
	Create(ctx context.Context, location *model.Location) error
	Update(ctx context.Context, location *model.Location) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type locationRepositoryImpl struct {
	db *gorm.DB
}

func NewLocationRepository(db *gorm.DB) LocationRepository {
	return &locationRepositoryImpl{db: db}
}

func (r *locationRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *locationRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Location, int64, error) {
	var locations []model.Location
	var total int64

	query := r.withContext(ctx).Model(&model.Location{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Location{}, 0, nil
	}

	// Apply filters
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("code ILIKE ? OR name ILIKE ? OR address ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Preload bujp relation
	query = query.Preload("Bujp", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "code", "name")
	})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&locations).Error
	return locations, total, err
}

func (r *locationRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Location, error) {
	var location model.Location
	if err := r.withContext(ctx).Preload("Bujp").Where("id = ?", id).First(&location).Error; err != nil {
		return nil, err
	}
	return &location, nil
}

func (r *locationRepositoryImpl) FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Location, error) {
	var locations []model.Location
	err := r.withContext(ctx).Where("bujp_id = ?", bujpID).Find(&locations).Error
	return locations, err
}

func (r *locationRepositoryImpl) FindByCode(ctx context.Context, code string) (*model.Location, error) {
	var location model.Location
	if err := r.withContext(ctx).Where("code = ?", code).First(&location).Error; err != nil {
		return nil, err
	}
	return &location, nil
}

func (r *locationRepositoryImpl) Create(ctx context.Context, location *model.Location) error {
	return r.withContext(ctx).Create(location).Error
}

func (r *locationRepositoryImpl) Update(ctx context.Context, location *model.Location) error {
	// Use Updates() instead of Save() for better performance
	return r.withContext(ctx).Model(location).Updates(location).Error
}

func (r *locationRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Location{}).Error
}
