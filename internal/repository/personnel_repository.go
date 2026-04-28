package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PersonnelRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Personnel, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Personnel, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Personnel, error)
	FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Personnel, error)
	FindByIDNumber(ctx context.Context, idNumber string) (*model.Personnel, error)
	Create(ctx context.Context, personnel *model.Personnel) error
	Update(ctx context.Context, personnel *model.Personnel) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type personnelRepositoryImpl struct {
	db *gorm.DB
}

func NewPersonnelRepository(db *gorm.DB) PersonnelRepository {
	return &personnelRepositoryImpl{db: db}
}

func (r *personnelRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *personnelRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Personnel, int64, error) {
	var personnels []model.Personnel
	var total int64

	query := r.withContext(ctx).Model(&model.Personnel{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Personnel{}, 0, nil
	}

	// Apply filters
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("id_number ILIKE ? OR full_name ILIKE ? OR phone ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if unassigned, ok := filters["unassigned_only"].(bool); ok && unassigned {
		// Exclude personnel that already have an active assignment
		query = query.Where("id NOT IN (?)",
			r.db.Table("assignments").
				Select("personnel_id").
				Where("status = ?", "active"),
		)
	}

	query = query.Preload("Bujp", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "code", "name")
	}).Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "email", "full_name", "role")
	})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&personnels).Error
	return personnels, total, err
}

func (r *personnelRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Personnel, error) {
	var personnel model.Personnel
	if err := r.withContext(ctx).Preload("Bujp").Preload("User").Where("id = ?", id).First(&personnel).Error; err != nil {
		return nil, err
	}
	return &personnel, nil
}
func (r *personnelRepositoryImpl) FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Personnel, error) {
	var personnel model.Personnel
	if err := r.withContext(ctx).Where("user_id = ?", userID).First(&personnel).Error; err != nil {
		return nil, err
	}
	return &personnel, nil
}
func (r *personnelRepositoryImpl) FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.Personnel, error) {
	var personnels []model.Personnel
	err := r.withContext(ctx).Preload("User").Where("bujp_id = ?", bujpID).Find(&personnels).Error
	return personnels, err
}

func (r *personnelRepositoryImpl) FindByIDNumber(ctx context.Context, idNumber string) (*model.Personnel, error) {
	var personnel model.Personnel
	if err := r.withContext(ctx).Where("id_number = ?", idNumber).First(&personnel).Error; err != nil {
		return nil, err
	}
	return &personnel, nil
}

func (r *personnelRepositoryImpl) Create(ctx context.Context, personnel *model.Personnel) error {
	return r.withContext(ctx).Create(personnel).Error
}

func (r *personnelRepositoryImpl) Update(ctx context.Context, personnel *model.Personnel) error {
	return r.withContext(ctx).Save(personnel).Error
}

func (r *personnelRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// Hard delete: skip soft-delete behaviour so the row is actually removed.
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Personnel{}).Error
}
