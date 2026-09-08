package repository

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/ganiramadhan/ganipedia/backend/internal/errors"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttendanceCorrectionRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrection, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AttendanceCorrection, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceCorrection, int64, error)
	FindPending(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrection, int64, error)
	FindApprovedByDateRange(ctx context.Context, bujpID uuid.UUID, start, end time.Time) ([]model.AttendanceCorrection, error)
	Create(ctx context.Context, correction *model.AttendanceCorrection) error
	Update(ctx context.Context, correction *model.AttendanceCorrection) error
	Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type attendanceCorrectionRepositoryImpl struct {
	db *gorm.DB
}

func NewAttendanceCorrectionRepository(db *gorm.DB) AttendanceCorrectionRepository {
	return &attendanceCorrectionRepositoryImpl{db: db}
}

func (r *attendanceCorrectionRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *attendanceCorrectionRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrection, int64, error) {
	db := r.withContext(ctx)

	var corrections []model.AttendanceCorrection
	var total int64

	query := db.Model(&model.AttendanceCorrection{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.AttendanceCorrection{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = attendance_corrections.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	// Apply filters
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("personnel_id = ?", personnelID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if correctionType, ok := filters["correction_type"].(string); ok && correctionType != "" {
		query = query.Where("correction_type = ?", correctionType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperrors.Internal("Failed to count attendance corrections")
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Approver").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&corrections).Error

	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve attendance corrections")
	}

	return corrections, total, nil
}

func (r *attendanceCorrectionRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.AttendanceCorrection, error) {
	db := r.withContext(ctx)

	var correction model.AttendanceCorrection
	err := db.
		Preload("Personnel").
		Preload("Approver").
		First(&correction, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("Attendance correction not found")
		}
		return nil, apperrors.Internal("Failed to retrieve attendance correction")
	}

	return &correction, nil
}

func (r *attendanceCorrectionRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceCorrection, int64, error) {
	db := r.withContext(ctx)

	var corrections []model.AttendanceCorrection
	var total int64

	query := db.Model(&model.AttendanceCorrection{}).Where("personnel_id = ?", personnelID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperrors.Internal("Failed to count personnel corrections")
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Approver").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&corrections).Error

	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve personnel corrections")
	}

	return corrections, total, nil
}

func (r *attendanceCorrectionRepositoryImpl) FindPending(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrection, int64, error) {
	db := r.withContext(ctx)

	var corrections []model.AttendanceCorrection
	var total int64

	if forceEmpty, _ := filters["force_empty"].(bool); forceEmpty {
		return corrections, 0, nil
	}

	query := db.Model(&model.AttendanceCorrection{}).Where("attendance_corrections.status = ?", "pending")

	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.
			Joins("JOIN personnels ON personnels.id = attendance_corrections.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperrors.Internal("Failed to count pending corrections")
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Order("attendance_corrections.created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&corrections).Error

	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve pending corrections")
	}

	return corrections, total, nil
}

func (r *attendanceCorrectionRepositoryImpl) FindApprovedByDateRange(ctx context.Context, bujpID uuid.UUID, start, end time.Time) ([]model.AttendanceCorrection, error) {
	db := r.withContext(ctx)

	var corrections []model.AttendanceCorrection
	query := db.Model(&model.AttendanceCorrection{}).
		Where("attendance_corrections.status = ?", "approved").
		Where("attendance_corrections.correction_date BETWEEN ? AND ?", start, end)

	if bujpID != uuid.Nil {
		query = query.
			Joins("JOIN personnels ON personnels.id = attendance_corrections.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	if err := query.
		Order("attendance_corrections.correction_date ASC").
		Find(&corrections).Error; err != nil {
		return nil, apperrors.Internal("Failed to retrieve approved corrections")
	}

	return corrections, nil
}

func (r *attendanceCorrectionRepositoryImpl) Create(ctx context.Context, correction *model.AttendanceCorrection) error {
	db := r.withContext(ctx)

	if err := db.Create(correction).Error; err != nil {
		return apperrors.Internal("Failed to create attendance correction")
	}

	return nil
}

func (r *attendanceCorrectionRepositoryImpl) Update(ctx context.Context, correction *model.AttendanceCorrection) error {
	db := r.withContext(ctx)

	result := db.Model(correction).Where("id = ?", correction.ID).Updates(correction)
	if result.Error != nil {
		return apperrors.Internal("Failed to update attendance correction")
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound("Attendance correction not found")
	}

	return nil
}

func (r *attendanceCorrectionRepositoryImpl) Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) error {
	db := r.withContext(ctx)

	now := time.Now()
	updates := map[string]interface{}{
		"status":         status,
		"approver_id":    approverID,
		"approved_at":    now,
		"approver_notes": notes,
	}

	result := db.Model(&model.AttendanceCorrection{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return apperrors.Internal("Failed to approve/reject attendance correction")
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound("Attendance correction not found")
	}

	return nil
}

func (r *attendanceCorrectionRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	db := r.withContext(ctx)

	result := db.Delete(&model.AttendanceCorrection{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.Internal("Failed to delete attendance correction")
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound("Attendance correction not found")
	}

	return nil
}
