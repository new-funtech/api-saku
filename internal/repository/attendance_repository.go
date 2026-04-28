package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttendanceRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Attendance, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Attendance, error)
	FindByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.Attendance, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Attendance, int64, error)
	FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Attendance, int64, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.Attendance, int64, error)
	Create(ctx context.Context, attendance *model.Attendance) error
	Update(ctx context.Context, attendance *model.Attendance) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type attendanceRepositoryImpl struct {
	db *gorm.DB
}

func NewAttendanceRepository(db *gorm.DB) AttendanceRepository {
	return &attendanceRepositoryImpl{db: db}
}

func (r *attendanceRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *attendanceRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Attendance, int64, error) {
	db := r.withContext(ctx)

	var attendances []model.Attendance
	var total int64

	query := db.Model(&model.Attendance{})

	// Tenant scoping (set by handler).
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Attendance{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = attendances.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	// Apply filters
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("personnel_id = ?", personnelID)
	}
	if locationID, ok := filters["location_id"].(uuid.UUID); ok && locationID != uuid.Nil {
		query = query.Where("location_id = ?", locationID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if date, ok := filters["date"].(time.Time); ok && !date.IsZero() {
		query = query.Where("date = ?", date)
	}
	if startDate, ok := filters["start_date"].(time.Time); ok && !startDate.IsZero() {
		query = query.Where("date >= ?", startDate)
	}
	if endDate, ok := filters["end_date"].(time.Time); ok && !endDate.IsZero() {
		query = query.Where("date <= ?", endDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Assignment").
		Order("date DESC, check_in DESC").
		Offset(offset).
		Limit(limit).
		Find(&attendances).Error

	return attendances, total, err
}

func (r *attendanceRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Attendance, error) {
	db := r.withContext(ctx)

	var attendance model.Attendance
	err := db.
		Preload("Personnel").
		Preload("Location").
		Preload("Assignment").
		First(&attendance, "id = ?", id).Error

	if err != nil {
		return nil, err
	}
	return &attendance, nil
}

func (r *attendanceRepositoryImpl) FindByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.Attendance, error) {
	db := r.withContext(ctx)

	var attendance model.Attendance
	err := db.
		Where("personnel_id = ? AND date = ?", personnelID, date).
		Preload("Personnel").
		Preload("Location").
		Preload("Assignment").
		First(&attendance).Error

	if err != nil {
		return nil, err
	}
	return &attendance, nil
}

func (r *attendanceRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Attendance, int64, error) {
	db := r.withContext(ctx)

	var attendances []model.Attendance
	var total int64

	query := db.Model(&model.Attendance{}).Where("personnel_id = ?", personnelID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Preload("Assignment").
		Order("date DESC").
		Offset(offset).
		Limit(limit).
		Find(&attendances).Error

	return attendances, total, err
}

func (r *attendanceRepositoryImpl) FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Attendance, int64, error) {
	db := r.withContext(ctx)

	var attendances []model.Attendance
	var total int64

	query := db.Model(&model.Attendance{}).Where("location_id = ?", locationID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Order("date DESC").
		Offset(offset).
		Limit(limit).
		Find(&attendances).Error

	return attendances, total, err
}

func (r *attendanceRepositoryImpl) FindByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.Attendance, int64, error) {
	db := r.withContext(ctx)

	var attendances []model.Attendance
	var total int64

	query := db.Model(&model.Attendance{}).
		Where("date >= ? AND date <= ?", startDate, endDate)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Location").
		Order("date DESC").
		Offset(offset).
		Limit(limit).
		Find(&attendances).Error

	return attendances, total, err
}

func (r *attendanceRepositoryImpl) Create(ctx context.Context, attendance *model.Attendance) error {
	db := r.withContext(ctx)

	return db.Create(attendance).Error
}

func (r *attendanceRepositoryImpl) Update(ctx context.Context, attendance *model.Attendance) error {
	db := r.withContext(ctx)

	result := db.Model(attendance).Where("id = ?", attendance.ID).Updates(attendance)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attendance not found")
	}
	return nil
}

func (r *attendanceRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	db := r.withContext(ctx)

	result := db.Delete(&model.Attendance{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attendance not found")
	}
	return nil
}
