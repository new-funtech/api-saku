package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MonthlyReportRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.MonthlyReport, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.MonthlyReport, error)
	FindByBujpAndPeriod(ctx context.Context, bujpID uuid.UUID, period string) (*model.MonthlyReport, error)
	ComputeStats(ctx context.Context, bujpID uuid.UUID, periodStart, periodEnd time.Time, period string) (*MonthlyReportStats, error)
	Create(ctx context.Context, report *model.MonthlyReport) error
	Update(ctx context.Context, report *model.MonthlyReport) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// MonthlyReportStats is the aggregated counters for a single (bujp, period).
type MonthlyReportStats struct {
	TotalGuards     int
	TotalLocations  int
	TotalPresent    int
	TotalPermission int
	TotalSick       int
	TotalAbsent     int
	TotalPatrols    int
	TotalPayroll    float64
}

type monthlyReportRepositoryImpl struct {
	db *gorm.DB
}

func NewMonthlyReportRepository(db *gorm.DB) MonthlyReportRepository {
	return &monthlyReportRepositoryImpl{db: db}
}

func (r *monthlyReportRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.MonthlyReport, int64, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var reports []model.MonthlyReport
	var total int64

	query := r.db.WithContext(ctxTimeout).Model(&model.MonthlyReport{})

	// Apply filters
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.MonthlyReport{}, 0, nil
	}
	if bujpUUID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpUUID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpUUID)
	} else if bujpID, ok := filters["bujp_id"].(string); ok && bujpID != "" {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if period, ok := filters["period"].(string); ok && period != "" {
		query = query.Where("period = ?", period)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get data with pagination and preload
	offset := (page - 1) * limit
	if err := query.
		Preload("Bujp").
		Preload("Creator").
		Order("period DESC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&reports).Error; err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *monthlyReportRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.MonthlyReport, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var report model.MonthlyReport
	if err := r.db.WithContext(ctxTimeout).
		Preload("Bujp").
		Preload("Creator").
		First(&report, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &report, nil
}

func (r *monthlyReportRepositoryImpl) FindByBujpAndPeriod(ctx context.Context, bujpID uuid.UUID, period string) (*model.MonthlyReport, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var report model.MonthlyReport
	if err := r.db.WithContext(ctxTimeout).
		Where("bujp_id = ? AND period = ?", bujpID, period).
		First(&report).Error; err != nil {
		return nil, err
	}

	return &report, nil
}

func (r *monthlyReportRepositoryImpl) Create(ctx context.Context, report *model.MonthlyReport) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Create(report).Error
}

func (r *monthlyReportRepositoryImpl) Update(ctx context.Context, report *model.MonthlyReport) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Save(report).Error
}

func (r *monthlyReportRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.db.WithContext(ctxTimeout).Delete(&model.MonthlyReport{}, "id = ?", id).Error
}

// ComputeStats aggregates personnel/attendance/patrol/payroll counts for the
// given BUJP and date window. Each counter runs as an independent SQL query
// because the underlying tables share no compatible join shape.
func (r *monthlyReportRepositoryImpl) ComputeStats(ctx context.Context, bujpID uuid.UUID, periodStart, periodEnd time.Time, period string) (*MonthlyReportStats, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db := r.db.WithContext(ctxTimeout)

	stats := &MonthlyReportStats{}

	// Total active guards under this BUJP.
	var guards int64
	if err := db.Table("personnels").
		Where("bujp_id = ? AND status = ?", bujpID, "active").
		Count(&guards).Error; err != nil {
		return nil, err
	}
	stats.TotalGuards = int(guards)

	// Total locations registered to this BUJP.
	var locations int64
	if err := db.Table("locations").
		Where("bujp_id = ?", bujpID).
		Count(&locations).Error; err != nil {
		return nil, err
	}
	stats.TotalLocations = int(locations)

	// Attendance counters grouped by status. We count rows per status in a
	// single GROUP BY query to avoid four separate scans.
	type attRow struct {
		Status string
		Total  int
	}
	var attRows []attRow
	if err := db.Table("attendances").
		Select("attendances.status AS status, COUNT(*) AS total").
		Joins("JOIN personnels ON personnels.id = attendances.personnel_id").
		Where("personnels.bujp_id = ? AND attendances.date BETWEEN ? AND ?", bujpID, periodStart, periodEnd).
		Group("attendances.status").
		Scan(&attRows).Error; err != nil {
		return nil, err
	}
	for _, row := range attRows {
		switch row.Status {
		case "present", "late":
			stats.TotalPresent += row.Total
		case "permission":
			stats.TotalPermission += row.Total
		case "sick":
			stats.TotalSick += row.Total
		case "absent":
			stats.TotalAbsent += row.Total
		}
	}

	// Patrol count for personnel within this BUJP for the period.
	var patrols int64
	if err := db.Table("patrols").
		Joins("JOIN personnels ON personnels.id = patrols.personnel_id").
		Where("personnels.bujp_id = ? AND patrols.created_at BETWEEN ? AND ?", bujpID, periodStart, periodEnd).
		Count(&patrols).Error; err != nil {
		return nil, err
	}
	stats.TotalPatrols = int(patrols)

	// Sum of net salary for the period.
	var totalPayroll *float64
	if err := db.Table("payrolls").
		Select("COALESCE(SUM(net_salary), 0)").
		Where("bujp_id = ? AND period = ?", bujpID, period).
		Scan(&totalPayroll).Error; err != nil {
		return nil, err
	}
	if totalPayroll != nil {
		stats.TotalPayroll = *totalPayroll
	}

	return stats, nil
}
