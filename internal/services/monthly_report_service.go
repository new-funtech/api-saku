package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MonthlyReportService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.MonthlyReportResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.MonthlyReportResponse, error)
	Create(ctx context.Context, req *model.CreateMonthlyReportRequest, createdBy *uuid.UUID) (*model.MonthlyReportResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateMonthlyReportRequest) (*model.MonthlyReportResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Generate(ctx context.Context, bujpID uuid.UUID, period string, force bool, createdBy *uuid.UUID) (*model.MonthlyReportResponse, error)
}

type monthlyreportServiceImpl struct {
	repo repository.MonthlyReportRepository
}

func NewMonthlyReportService(repo repository.MonthlyReportRepository) MonthlyReportService {
	return &monthlyreportServiceImpl{repo: repo}
}

func (s *monthlyreportServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.MonthlyReportResponse, int64, error) {
	reports, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.MonthlyReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, toMonthlyReportResponse(&report))
	}

	return responses, total, nil
}

func (s *monthlyreportServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.MonthlyReportResponse, error) {
	report, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("monthly report not found")
		}
		return nil, err
	}

	response := toMonthlyReportResponse(report)
	return &response, nil
}

func (s *monthlyreportServiceImpl) Create(ctx context.Context, req *model.CreateMonthlyReportRequest, createdBy *uuid.UUID) (*model.MonthlyReportResponse, error) {
	// Check duplicate bujp + period
	if existing, _ := s.repo.FindByBujpAndPeriod(ctx, req.BujpID, req.Period); existing != nil {
		return nil, fmt.Errorf("monthly report for this BUJP and period already exists")
	}

	report := &model.MonthlyReport{
		BujpID:          req.BujpID,
		Period:          req.Period,
		TotalGuards:     req.TotalGuards,
		TotalLocations:  req.TotalLocations,
		TotalPresent:    req.TotalPresent,
		TotalPermission: req.TotalPermission,
		TotalSick:       req.TotalSick,
		TotalAbsent:     req.TotalAbsent,
		TotalPatrols:    req.TotalPatrols,
		TotalPayroll:    req.TotalPayroll,
		ReportFile:      req.ReportFile,
		CreatedBy:       createdBy,
	}

	if err := s.repo.Create(ctx, report); err != nil {
		return nil, err
	}

	// Reload to get relations
	created, err := s.repo.FindByID(ctx, report.ID)
	if err != nil {
		return nil, err
	}

	response := toMonthlyReportResponse(created)
	return &response, nil
}

func (s *monthlyreportServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateMonthlyReportRequest) (*model.MonthlyReportResponse, error) {
	report, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("monthly report not found")
		}
		return nil, err
	}

	// Update fields
	if req.Period != nil {
		report.Period = *req.Period
	}
	if req.TotalGuards != nil {
		report.TotalGuards = *req.TotalGuards
	}
	if req.TotalLocations != nil {
		report.TotalLocations = *req.TotalLocations
	}
	if req.TotalPresent != nil {
		report.TotalPresent = *req.TotalPresent
	}
	if req.TotalPermission != nil {
		report.TotalPermission = *req.TotalPermission
	}
	if req.TotalSick != nil {
		report.TotalSick = *req.TotalSick
	}
	if req.TotalAbsent != nil {
		report.TotalAbsent = *req.TotalAbsent
	}
	if req.TotalPatrols != nil {
		report.TotalPatrols = *req.TotalPatrols
	}
	if req.TotalPayroll != nil {
		report.TotalPayroll = *req.TotalPayroll
	}
	if req.ReportFile != nil {
		report.ReportFile = req.ReportFile
	}

	if err := s.repo.Update(ctx, report); err != nil {
		return nil, err
	}

	// Reload to get updated relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toMonthlyReportResponse(updated)
	return &response, nil
}

func (s *monthlyreportServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("monthly report not found")
		}
		return err
	}

	return s.repo.Delete(ctx, id)
}

// Generate computes the monthly aggregates for a (bujp, period) combo and
// upserts the report. When force=false and a report already exists for the
// same key, it returns a conflict-like error so the caller can confirm.
func (s *monthlyreportServiceImpl) Generate(ctx context.Context, bujpID uuid.UUID, period string, force bool, createdBy *uuid.UUID) (*model.MonthlyReportResponse, error) {
	if bujpID == uuid.Nil {
		return nil, errors.New("bujp_id wajib diisi")
	}
	if len(period) != 7 || period[4] != '-' {
		return nil, errors.New("period harus dalam format YYYY-MM")
	}
	periodStart, err := time.Parse("2006-01-02", period+"-01")
	if err != nil {
		return nil, errors.New("period tidak valid")
	}
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	existing, _ := s.repo.FindByBujpAndPeriod(ctx, bujpID, period)
	if existing != nil && !force {
		return nil, errors.New("laporan untuk BUJP dan periode ini sudah ada. Gunakan force=true untuk regenerate")
	}

	stats, err := s.repo.ComputeStats(ctx, bujpID, periodStart, periodEnd, period)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung statistik: %w", err)
	}

	if existing != nil {
		existing.TotalGuards = stats.TotalGuards
		existing.TotalLocations = stats.TotalLocations
		existing.TotalPresent = stats.TotalPresent
		existing.TotalPermission = stats.TotalPermission
		existing.TotalSick = stats.TotalSick
		existing.TotalAbsent = stats.TotalAbsent
		existing.TotalPatrols = stats.TotalPatrols
		existing.TotalPayroll = stats.TotalPayroll
		existing.CreatedBy = createdBy
		// Nil out preloaded relations to avoid GORM Save+Preload FK revert.
		existing.Bujp = nil
		existing.Creator = nil
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		reloaded, err := s.repo.FindByID(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		resp := toMonthlyReportResponse(reloaded)
		return &resp, nil
	}

	report := &model.MonthlyReport{
		BujpID:          bujpID,
		Period:          period,
		TotalGuards:     stats.TotalGuards,
		TotalLocations:  stats.TotalLocations,
		TotalPresent:    stats.TotalPresent,
		TotalPermission: stats.TotalPermission,
		TotalSick:       stats.TotalSick,
		TotalAbsent:     stats.TotalAbsent,
		TotalPatrols:    stats.TotalPatrols,
		TotalPayroll:    stats.TotalPayroll,
		CreatedBy:       createdBy,
	}
	if err := s.repo.Create(ctx, report); err != nil {
		return nil, err
	}
	reloaded, err := s.repo.FindByID(ctx, report.ID)
	if err != nil {
		return nil, err
	}
	resp := toMonthlyReportResponse(reloaded)
	return &resp, nil
}

func toMonthlyReportResponse(report *model.MonthlyReport) model.MonthlyReportResponse {
	response := model.MonthlyReportResponse{
		ID:              report.ID,
		BujpID:          report.BujpID,
		Period:          report.Period,
		TotalGuards:     report.TotalGuards,
		TotalLocations:  report.TotalLocations,
		TotalPresent:    report.TotalPresent,
		TotalPermission: report.TotalPermission,
		TotalSick:       report.TotalSick,
		TotalAbsent:     report.TotalAbsent,
		TotalPatrols:    report.TotalPatrols,
		TotalPayroll:    report.TotalPayroll,
		ReportFile:      report.ReportFile,
		CreatedBy:       report.CreatedBy,
		CreatedAt:       report.CreatedAt,
		UpdatedAt:       report.UpdatedAt,
	}

	if report.Bujp != nil {
		response.Bujp = &model.BujpResponse{
			ID:     report.Bujp.ID,
			Code:   report.Bujp.Code,
			Name:   report.Bujp.Name,
			Status: report.Bujp.Status,
		}
	}

	if report.Creator != nil {
		response.Creator = &model.UserResponse{
			ID:       report.Creator.ID,
			FullName: report.Creator.FullName,
			Email:    report.Creator.Email,
		}
	}

	return response
}
