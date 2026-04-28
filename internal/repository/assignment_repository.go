package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AssignmentRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Assignment, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Assignment, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Assignment, int64, error)
	FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Assignment, int64, error)
	FindActiveByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.Assignment, error)
	FindCurrentByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.Assignment, error)
	Create(ctx context.Context, assignment *model.Assignment) error
	Update(ctx context.Context, assignment *model.Assignment) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByPersonnelID(ctx context.Context, personnelID uuid.UUID) error
}

type assignmentRepositoryImpl struct {
	db *gorm.DB
}

func NewAssignmentRepository(db *gorm.DB) AssignmentRepository {
	return &assignmentRepositoryImpl{db: db}
}

func (r *assignmentRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *assignmentRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Assignment, int64, error) {
	db := r.withContext(ctx)

	var assignments []model.Assignment
	var total int64

	query := db.Model(&model.Assignment{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Assignment{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = assignments.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	// Apply filters (always qualify columns; we may have joined `personnels`
	// which also has `status`, `notes`, etc. -> avoids 42702 ambiguous column)
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("assignments.personnel_id = ?", personnelID)
	}
	if locationID, ok := filters["location_id"].(uuid.UUID); ok && locationID != uuid.Nil {
		query = query.Where("assignments.location_id = ?", locationID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("assignments.status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("assignments.notes ILIKE ?", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Shift").
		Preload("Creator").
		Order("assignments.start_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&assignments).Error

	return assignments, total, err
}

func (r *assignmentRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Assignment, error) {
	db := r.withContext(ctx)

	var assignment model.Assignment
	err := db.
		Preload("Personnel").
		Preload("Location").
		Preload("Shift").
		Preload("Creator").
		First(&assignment, "id = ?", id).Error

	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *assignmentRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Assignment, int64, error) {
	db := r.withContext(ctx)

	var assignments []model.Assignment
	var total int64

	query := db.Model(&model.Assignment{}).Where("personnel_id = ?", personnelID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Shift").
		Order("start_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&assignments).Error

	return assignments, total, err
}

func (r *assignmentRepositoryImpl) FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Assignment, int64, error) {
	db := r.withContext(ctx)

	var assignments []model.Assignment
	var total int64

	query := db.Model(&model.Assignment{}).Where("location_id = ?", locationID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Shift").
		Order("start_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&assignments).Error

	return assignments, total, err
}

func (r *assignmentRepositoryImpl) FindActiveByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.Assignment, error) {
	db := r.withContext(ctx)

	var assignment model.Assignment
	err := db.
		Where("personnel_id = ? AND status = ?", personnelID, "active").
		Where("start_date <= ? AND (end_date IS NULL OR end_date >= ?)", time.Now(), time.Now()).
		Preload("Personnel").
		Preload("Location").
		Preload("Location.Bujp").
		Preload("Shift").
		First(&assignment).Error

	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *assignmentRepositoryImpl) FindCurrentByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.Assignment, error) {
	return r.FindActiveByPersonnelID(ctx, personnelID)
}

func (r *assignmentRepositoryImpl) Create(ctx context.Context, assignment *model.Assignment) error {
	db := r.withContext(ctx)

	return db.Create(assignment).Error
}

func (r *assignmentRepositoryImpl) Update(ctx context.Context, assignment *model.Assignment) error {
	db := r.withContext(ctx)

	// Use Save() so all fields persist, including changes to location_id /
	// shift_id that Updates() would skip when GORM treats them as zero.
	result := db.Save(assignment)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("assignment not found")
	}
	return nil
}

func (r *assignmentRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	db := r.withContext(ctx)

	result := db.Delete(&model.Assignment{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("assignment not found")
	}
	return nil
}

// DeleteByPersonnelID removes every assignment tied to the given personnel.
// Used to cascade deletions when a user (and their personnel record) are
// being removed; missing rows are not treated as an error.
func (r *assignmentRepositoryImpl) DeleteByPersonnelID(ctx context.Context, personnelID uuid.UUID) error {
	return r.withContext(ctx).
		Where("personnel_id = ?", personnelID).
		Delete(&model.Assignment{}).Error
}
