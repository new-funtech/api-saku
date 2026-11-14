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
	// Recap counts each person's attendance, leave and overtime in a date range (company scoped).
	Recap(ctx context.Context, q RecapQuery) ([]model.AttendanceRecapRow, error)
	Create(ctx context.Context, report *model.MonthlyReport) error
	Update(ctx context.Context, report *model.MonthlyReport) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// RecapQuery selects the people and the days of a recap. BujpID is always set by the caller's
// session (or, for central roles, an explicit company); it is what keeps the recap inside a tenant.
type RecapQuery struct {
	BujpID      uuid.UUID
	From, To    time.Time
	LocationID  uuid.UUID // optional
	PersonnelID uuid.UUID // optional
	Search      string    // optional: name, NIK or NIP
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

func (r *monthlyReportRepositoryImpl) Recap(ctx context.Context, q RecapQuery) ([]model.AttendanceRecapRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Placeholders appear in this order in the SQL below: overtime dates, join dates, [office],
	// company, then the optional where parts.
	args := []interface{}{q.From, q.To, q.From, q.To}
	attendanceFilter := ""
	if q.LocationID != uuid.Nil {
		attendanceFilter = " AND a.location_id = ?"
		args = append(args, q.LocationID)
	}
	args = append(args, q.BujpID)
	where := ""
	if q.PersonnelID != uuid.Nil {
		where += " AND p.id = ?"
		args = append(args, q.PersonnelID)
	}
	if q.LocationID != uuid.Nil {
		where += " AND (EXISTS (SELECT 1 FROM assignments s WHERE s.personnel_id = p.id AND s.location_id = ?) OR EXISTS (SELECT 1 FROM attendances x WHERE x.personnel_id = p.id AND x.location_id = ? AND x.date BETWEEN ? AND ?))"
		args = append(args, q.LocationID, q.LocationID, q.From, q.To)
	}
	if q.Search != "" {
		like := "%" + q.Search + "%"
		where += " AND (p.full_name ILIKE ? OR p.id_number ILIKE ? OR p.n_ip ILIKE ?)"
		args = append(args, like, like, like)
	}

	sql := `
SELECT p.id AS personnel_id, p.full_name, p.id_number, p.n_ip AS nip,
  COUNT(a.id) FILTER (WHERE a.status IN ('present','late')) AS hadir,
  COUNT(a.id) FILTER (WHERE a.status = 'late') AS telat,
  COUNT(a.id) FILTER (WHERE a.status = 'absent') AS tidak_hadir,
  COUNT(a.id) FILTER (WHERE a.status IN ('permission','sick','leave')) AS izin,
  COALESCE((SELECT SUM(o.total_hours) FROM overtimes o WHERE o.personnel_id = p.id AND o.status = 'approved' AND o.date BETWEEN ? AND ?), 0) AS lembur_jam,
  COALESCE(SUM(a.work_duration), 0) AS jam_kerja_menit
FROM personnels p
LEFT JOIN attendances a ON a.personnel_id = p.id AND a.date BETWEEN ? AND ?` + attendanceFilter + `
WHERE p.bujp_id = ?` + where + `
GROUP BY p.id
ORDER BY p.full_name`

	var rows []model.AttendanceRecapRow
	err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error
	return rows, err
}
