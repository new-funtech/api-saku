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
	CountByStatus(ctx context.Context, filters map[string]interface{}) (map[string]int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Attendance, error)
	FindByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.Attendance, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Attendance, int64, error)
	FindByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.Attendance, int64, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.Attendance, int64, error)
	Create(ctx context.Context, attendance *model.Attendance) error
	// ExistsByPersonnelAndDate is the cheap duplicate check (no relations loaded).
	ExistsByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (bool, error)
	// Insert writes one row with a single statement (no wrapping transaction).
	Insert(ctx context.Context, attendance *model.Attendance) error
	// FindOpenForCheckout returns the rows of the given dates with the shift and office loaded in one query.
	FindOpenForCheckout(ctx context.Context, personnelID uuid.UUID, from, to time.Time) ([]model.Attendance, error)
	// CompleteCheckOut sets the check-out only if it is still empty; false when it was already set.
	CompleteCheckOut(ctx context.Context, id uuid.UUID, c CheckOutUpdate) (bool, error)
	Update(ctx context.Context, attendance *model.Attendance) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CheckOutUpdate is everything a check-out writes.
type CheckOutUpdate struct {
	At           time.Time
	Latitude     *string
	Longitude    *string
	Photo        *string
	Document     *string
	Notes        *string
	WorkDuration int
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

// scopedAttendances applies the shared list filters. empty is true when the
// caller must see nothing (an unscoped non-admin).
func scopedAttendances(db *gorm.DB, filters map[string]interface{}) (query *gorm.DB, empty bool) {
	query = db.Model(&model.Attendance{})
	if v, ok := filters["force_empty"].(bool); ok && v {
		return query, true
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = attendances.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("attendances.personnel_id = ?", personnelID)
	}
	if locationID, ok := filters["location_id"].(uuid.UUID); ok && locationID != uuid.Nil {
		query = query.Where("attendances.location_id = ?", locationID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("attendances.status = ?", status)
	}
	if date, ok := filters["date"].(time.Time); ok && !date.IsZero() {
		query = query.Where("attendances.date = ?", date)
	}
	if startDate, ok := filters["start_date"].(time.Time); ok && !startDate.IsZero() {
		query = query.Where("attendances.date >= ?", startDate)
	}
	if endDate, ok := filters["end_date"].(time.Time); ok && !endDate.IsZero() {
		query = query.Where("attendances.date <= ?", endDate)
	}
	return query, false
}

// CountByStatus returns attendance counts per status in a single query.
func (r *attendanceRepositoryImpl) CountByStatus(ctx context.Context, filters map[string]interface{}) (map[string]int64, error) {
	out := map[string]int64{}
	query, empty := scopedAttendances(r.withContext(ctx), filters)
	if empty {
		return out, nil
	}
	var rows []struct {
		Status string
		Total  int64
	}
	if err := query.Select("attendances.status AS status, COUNT(*) AS total").
		Group("attendances.status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.Status] = rw.Total
	}
	return out, nil
}

func (r *attendanceRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Attendance, int64, error) {
	db := r.withContext(ctx)

	var attendances []model.Attendance
	var total int64

	query, empty := scopedAttendances(db, filters)
	if empty {
		return []model.Attendance{}, 0, nil
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
		Preload("Assignment.Shift").
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

func (r *attendanceRepositoryImpl) ExistsByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (bool, error) {
	var n int64
	err := r.withContext(ctx).Model(&model.Attendance{}).
		Where("personnel_id = ? AND date = ?", personnelID, date).Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *attendanceRepositoryImpl) Insert(ctx context.Context, attendance *model.Attendance) error {
	return r.withContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).Create(attendance).Error
}

func (r *attendanceRepositoryImpl) FindOpenForCheckout(ctx context.Context, personnelID uuid.UUID, from, to time.Time) ([]model.Attendance, error) {
	var rows []model.Attendance
	err := r.withContext(ctx).
		Joins("Location").
		Joins("Assignment").
		Joins("Assignment.Shift").
		Where("attendances.personnel_id = ? AND attendances.date >= ? AND attendances.date <= ?", personnelID, from, to).
		Order("attendances.date DESC").
		Find(&rows).Error
	return rows, err
}

func (r *attendanceRepositoryImpl) CompleteCheckOut(ctx context.Context, id uuid.UUID, c CheckOutUpdate) (bool, error) {
	updates := map[string]interface{}{
		"check_out":           c.At,
		"check_out_latitude":  c.Latitude,
		"check_out_longitude": c.Longitude,
		"work_duration":       c.WorkDuration,
		"updated_at":          time.Now(),
	}
	if c.Photo != nil {
		updates["check_out_photo"] = *c.Photo
	}
	if c.Document != nil {
		updates["supporting_document"] = *c.Document
	}
	if c.Notes != nil {
		updates["notes"] = *c.Notes
	}
	res := r.withContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).
		Model(&model.Attendance{}).Where("id = ? AND check_out IS NULL", id).Updates(updates)
	return res.RowsAffected > 0, res.Error
}
