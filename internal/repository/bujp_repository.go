package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BujpRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Bujp, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Bujp, error)
	FindByCode(ctx context.Context, code string) (*model.Bujp, error)
	Create(ctx context.Context, bujp *model.Bujp) error
	Update(ctx context.Context, bujp *model.Bujp) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type bujpRepositoryImpl struct {
	db *gorm.DB
}

func NewBujpRepository(db *gorm.DB) BujpRepository {
	return &bujpRepositoryImpl{db: db}
}

func (r *bujpRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *bujpRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Bujp, int64, error) {
	var bujps []model.Bujp
	var total int64

	query := r.withContext(ctx).Model(&model.Bujp{})

	// Apply filters
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("code ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&bujps).Error
	return bujps, total, err
}

func (r *bujpRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Bujp, error) {
	var bujp model.Bujp
	if err := r.withContext(ctx).Where("id = ?", id).First(&bujp).Error; err != nil {
		return nil, err
	}
	return &bujp, nil
}

func (r *bujpRepositoryImpl) FindByCode(ctx context.Context, code string) (*model.Bujp, error) {
	var bujp model.Bujp
	if err := r.withContext(ctx).Where("code = ?", code).First(&bujp).Error; err != nil {
		return nil, err
	}
	return &bujp, nil
}

func (r *bujpRepositoryImpl) Create(ctx context.Context, bujp *model.Bujp) error {
	return r.withContext(ctx).Create(bujp).Error
}

func (r *bujpRepositoryImpl) Update(ctx context.Context, bujp *model.Bujp) error {
	return r.withContext(ctx).Model(bujp).Updates(bujp).Error
}

func (r *bujpRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Bujp{}).Error
}
