package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PatrolRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Patrol, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Patrol, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Patrol, int64, error)
	FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Patrol, int64, error)
	FindByAttendanceID(ctx context.Context, attendanceID uuid.UUID) ([]model.Patrol, error)
	FindPending(ctx context.Context, page, limit int) ([]model.Patrol, int64, error)
	Create(ctx context.Context, patrol *model.Patrol) error
	Update(ctx context.Context, patrol *model.Patrol) error
	Validate(ctx context.Context, id uuid.UUID, validatorID uuid.UUID, status string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type patrolRepositoryImpl struct {
	db *gorm.DB
}

func NewPatrolRepository(db *gorm.DB) PatrolRepository {
	return &patrolRepositoryImpl{db: db}
}

func (r *patrolRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *patrolRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Patrol, int64, error) {
	db := r.withContext(ctx)

	var patrols []model.Patrol
	var total int64

	query := db.Model(&model.Patrol{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Patrol{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = patrols.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	// Apply filters
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("personnel_id = ?", personnelID)
	}
	if locationID, ok := filters["location_id"].(uuid.UUID); ok && locationID != uuid.Nil {
		query = query.Where("location_id = ?", locationID)
	}
	if validationStatus, ok := filters["validation_status"].(string); ok && validationStatus != "" {
		query = query.Where("validation_status = ?", validationStatus)
	}
	if date, ok := filters["date"].(time.Time); ok && !date.IsZero() {
		query = query.Where("date = ?", date)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Attendance").
		Preload("Validator").
		Order("patrol_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&patrols).Error

	return patrols, total, err
}

func (r *patrolRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Patrol, error) {
	db := r.withContext(ctx)

	var patrol model.Patrol
	err := db.
		Preload("Personnel").
		Preload("Location").
		Preload("Attendance").
		Preload("Validator").
		First(&patrol, "id = ?", id).Error

	if err != nil {
		return nil, err
	}
	return &patrol, nil
}

func (r *patrolRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Patrol, int64, error) {
	db := r.withContext(ctx)

	var patrols []model.Patrol
	var total int64

	query := db.Model(&model.Patrol{}).Where("personnel_id = ?", personnelID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Validator").
		Order("patrol_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&patrols).Error

	return patrols, total, err
}

func (r *patrolRepositoryImpl) FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Patrol, int64, error) {
	db := r.withContext(ctx)

	var patrols []model.Patrol
	var total int64

	query := db.Model(&model.Patrol{}).Where("location_id = ?", locationID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Order("patrol_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&patrols).Error

	return patrols, total, err
}

func (r *patrolRepositoryImpl) FindByAttendanceID(ctx context.Context, attendanceID uuid.UUID) ([]model.Patrol, error) {
	db := r.withContext(ctx)

	var patrols []model.Patrol
	err := db.
		Where("attendance_id = ?", attendanceID).
		Preload("Personnel").
		Preload("Location").
		Order("patrol_time ASC").
		Find(&patrols).Error

	return patrols, err
}

func (r *patrolRepositoryImpl) FindPending(ctx context.Context, page, limit int) ([]model.Patrol, int64, error) {
	db := r.withContext(ctx)

	var patrols []model.Patrol
	var total int64

	query := db.Model(&model.Patrol{}).Where("validation_status = ?", "pending")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Order("patrol_time ASC").
		Offset(offset).
		Limit(limit).
		Find(&patrols).Error

	return patrols, total, err
}

func (r *patrolRepositoryImpl) Create(ctx context.Context, patrol *model.Patrol) error {
	db := r.withContext(ctx)

	return db.Create(patrol).Error
}

func (r *patrolRepositoryImpl) Update(ctx context.Context, patrol *model.Patrol) error {
	db := r.withContext(ctx)

	result := db.Model(patrol).Where("id = ?", patrol.ID).Updates(patrol)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("patrol not found")
	}
	return nil
}

func (r *patrolRepositoryImpl) Validate(ctx context.Context, id uuid.UUID, validatorID uuid.UUID, status string) error {
	db := r.withContext(ctx)

	now := time.Now()
	updates := map[string]interface{}{
		"validation_status": status,
		"validator_id":      validatorID,
		"validated_at":      now,
	}

	result := db.Model(&model.Patrol{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("patrol not found")
	}
	return nil
}

func (r *patrolRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	db := r.withContext(ctx)

	result := db.Delete(&model.Patrol{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("patrol not found")
	}
	return nil
}
