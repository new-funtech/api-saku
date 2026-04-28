package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/mailer"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PayrollService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PayrollResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.PayrollResponse, error)
	Create(ctx context.Context, req *model.CreatePayrollRequest, createdBy *uuid.UUID) (*model.PayrollResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdatePayrollRequest) (*model.PayrollResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Generate(ctx context.Context, req *model.GeneratePayrollRequest, createdBy *uuid.UUID) (*model.GeneratePayrollResult, error)
}

type payrollServiceImpl struct {
	repo                repository.PayrollRepository
	personnelRepo       repository.PersonnelRepository
	attendanceRepo      repository.AttendanceRepository
	salaryComponentRepo repository.SalaryComponentRepository
}

func NewPayrollService(
	repo repository.PayrollRepository,
	personnelRepo repository.PersonnelRepository,
	attendanceRepo repository.AttendanceRepository,
	salaryComponentRepo repository.SalaryComponentRepository,
) PayrollService {
	return &payrollServiceImpl{
		repo:                repo,
		personnelRepo:       personnelRepo,
		attendanceRepo:      attendanceRepo,
		salaryComponentRepo: salaryComponentRepo,
	}
}

func (s *payrollServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PayrollResponse, int64, error) {
	payrolls, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.PayrollResponse, 0, len(payrolls))
	for _, payroll := range payrolls {
		responses = append(responses, toPayrollResponse(&payroll))
	}

	return responses, total, nil
}

func (s *payrollServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.PayrollResponse, error) {
	payroll, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payroll not found")
		}
		return nil, err
	}

	response := toPayrollResponse(payroll)
	return &response, nil
}

func (s *payrollServiceImpl) Create(ctx context.Context, req *model.CreatePayrollRequest, createdBy *uuid.UUID) (*model.PayrollResponse, error) {
	// Check duplicate personnel + period
	if existing, _ := s.repo.FindByPersonnelAndPeriod(ctx, req.PersonnelID, req.Period); existing != nil {
		return nil, fmt.Errorf("payroll for this personnel and period already exists")
	}

	// Calculate totals
	var totalAllowances, totalDeductions float64
	for _, detail := range req.Details {
		if detail.Amount > 0 {
			totalAllowances += detail.Amount
		} else {
			totalDeductions += -detail.Amount
		}
	}

	netSalary := req.BaseSalary + totalAllowances - totalDeductions

	// Set default status
	paymentStatus := constants.PaymentStatusPending
	if req.PaymentStatus != nil {
		paymentStatus = *req.PaymentStatus
	}

	// Parse payment date if provided
	var paymentDate *time.Time
	if req.PaymentDate != nil {
		parsed, err := utils.ParseDate(*req.PaymentDate)
		if err == nil {
			paymentDate = &parsed
		}
	}

	payroll := &model.Payroll{
		PersonnelID:     req.PersonnelID,
		BujpID:          req.BujpID,
		Period:          req.Period,
		BaseSalary:      req.BaseSalary,
		TotalAllowances: totalAllowances,
		TotalDeductions: totalDeductions,
		NetSalary:       netSalary,
		TotalPresent:    req.TotalPresent,
		TotalPermission: req.TotalPermission,
		TotalAbsent:     req.TotalAbsent,
		PaymentStatus:   paymentStatus,
		PaymentDate:     paymentDate,
		PaymentMethod:   req.PaymentMethod,
		CreatedBy:       createdBy,
	}

	// Build details
	details := make([]model.PayrollDetail, 0, len(req.Details))
	for _, d := range req.Details {
		details = append(details, model.PayrollDetail{
			SalaryComponentID: d.SalaryComponentID,
			Amount:            d.Amount,
			Notes:             d.Notes,
		})
	}

	// Create with transaction
	if err := s.repo.CreateWithDetails(ctx, payroll, details); err != nil {
		return nil, err
	}

	// Reload to get relations
	created, err := s.repo.FindByID(ctx, payroll.ID)
	if err != nil {
		return nil, err
	}

	response := toPayrollResponse(created)
	return &response, nil
}

func (s *payrollServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdatePayrollRequest) (*model.PayrollResponse, error) {
	payroll, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payroll not found")
		}
		return nil, err
	}

	// Update fields
	if req.Period != nil {
		payroll.Period = *req.Period
	}
	if req.BaseSalary != nil {
		payroll.BaseSalary = *req.BaseSalary
		// Recalculate net salary
		payroll.NetSalary = payroll.BaseSalary + payroll.TotalAllowances - payroll.TotalDeductions
	}
	if req.TotalPresent != nil {
		payroll.TotalPresent = *req.TotalPresent
	}
	if req.TotalPermission != nil {
		payroll.TotalPermission = *req.TotalPermission
	}
	if req.TotalAbsent != nil {
		payroll.TotalAbsent = *req.TotalAbsent
	}
	if req.PaymentStatus != nil {
		payroll.PaymentStatus = *req.PaymentStatus
	}
	if req.PaymentDate != nil {
		parsed, err := utils.ParseDate(*req.PaymentDate)
		if err == nil {
			payroll.PaymentDate = &parsed
		}
	}
	if req.PaymentMethod != nil {
		payroll.PaymentMethod = req.PaymentMethod
	}

	if err := s.repo.Update(ctx, payroll); err != nil {
		return nil, err
	}

	// Reload to get updated relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toPayrollResponse(updated)
	return &response, nil
}

func (s *payrollServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("payroll not found")
		}
		return err
	}

	return s.repo.Delete(ctx, id)
}

func toPayrollResponse(payroll *model.Payroll) model.PayrollResponse {
	response := model.PayrollResponse{
		ID:              payroll.ID,
		PersonnelID:     payroll.PersonnelID,
		BujpID:          payroll.BujpID,
		Period:          payroll.Period,
		BaseSalary:      payroll.BaseSalary,
		TotalAllowances: payroll.TotalAllowances,
		TotalDeductions: payroll.TotalDeductions,
		NetSalary:       payroll.NetSalary,
		TotalPresent:    payroll.TotalPresent,
		TotalPermission: payroll.TotalPermission,
		TotalAbsent:     payroll.TotalAbsent,
		PaymentStatus:   payroll.PaymentStatus,
		PaymentDate:     payroll.PaymentDate,
		PaymentMethod:   payroll.PaymentMethod,
		CreatedBy:       payroll.CreatedBy,
		CreatedAt:       payroll.CreatedAt,
		UpdatedAt:       payroll.UpdatedAt,
	}

	if payroll.Personnel != nil {
		response.Personnel = &model.PersonnelResponse{
			ID:       payroll.Personnel.ID,
			IDNumber: payroll.Personnel.IDNumber,
			FullName: payroll.Personnel.FullName,
			Phone:    payroll.Personnel.Phone,
			Status:   payroll.Personnel.Status,
		}
	}

	if payroll.Bujp != nil {
		response.Bujp = &model.BujpResponse{
			ID:     payroll.Bujp.ID,
			Code:   payroll.Bujp.Code,
			Name:   payroll.Bujp.Name,
			Status: payroll.Bujp.Status,
		}
	}

	if payroll.Creator != nil {
		response.Creator = &model.UserResponse{
			ID:       payroll.Creator.ID,
			FullName: payroll.Creator.FullName,
			Email:    payroll.Creator.Email,
		}
	}

	if len(payroll.PayrollDetails) > 0 {
		response.PayrollDetails = make([]model.PayrollDetailResponse, 0, len(payroll.PayrollDetails))
		for _, detail := range payroll.PayrollDetails {
			detailResp := model.PayrollDetailResponse{
				ID:                detail.ID,
				PayrollID:         detail.PayrollID,
				SalaryComponentID: detail.SalaryComponentID,
				Amount:            detail.Amount,
				Notes:             detail.Notes,
				CreatedAt:         detail.CreatedAt,
			}
			if detail.SalaryComponent != nil {
				detailResp.SalaryComponent = &model.SalaryComponentResponse{
					ID:                detail.SalaryComponent.ID,
					Code:              detail.SalaryComponent.Code,
					Name:              detail.SalaryComponent.Name,
					Type:              detail.SalaryComponent.Type,
					CalculationMethod: detail.SalaryComponent.CalculationMethod,
				}
			}
			response.PayrollDetails = append(response.PayrollDetails, detailResp)
		}
	}

	return response
}

// Generate ports laravel-backend PayrollController@generate.
// Pro-rates each personnel's base salary by attendance rate (present + permission)
// over total weekdays in the period, then applies BUJP salary components
// (fixed components are pro-rated by the same factor; percentage components
// run against the already pro-rated base).
func (s *payrollServiceImpl) Generate(ctx context.Context, req *model.GeneratePayrollRequest, createdBy *uuid.UUID) (*model.GeneratePayrollResult, error) {
	if req == nil || req.BujpID == uuid.Nil {
		return nil, fmt.Errorf("bujp_id wajib diisi")
	}
	periodStart, err := time.Parse(constants.PayrollPeriodFormat, req.Period)
	if err != nil {
		return nil, fmt.Errorf("period harus dalam format YYYY-MM")
	}
	periodEnd := periodStart.AddDate(0, 1, -1)

	personnels, err := s.personnelRepo.FindByBujpID(ctx, req.BujpID)
	if err != nil {
		return nil, err
	}
	if len(personnels) == 0 {
		return nil, fmt.Errorf("tidak ada personel aktif untuk BUJP ini")
	}

	// Count weekdays in period (Mon-Fri).
	workingDays := 0
	for d := periodStart; !d.After(periodEnd); d = d.AddDate(0, 0, 1) {
		w := d.Weekday()
		if w != time.Saturday && w != time.Sunday {
			workingDays++
		}
	}

	// Active salary components for this BUJP.
	componentFilters := map[string]interface{}{
		"bujp_id":   req.BujpID,
		"is_active": true,
	}
	components, _, err := s.salaryComponentRepo.FindAll(ctx, 1, 1000, componentFilters)
	if err != nil {
		return nil, err
	}

	result := &model.GeneratePayrollResult{}

	for i := range personnels {
		p := personnels[i]
		if p.Status != "active" {
			continue
		}

		existing, _ := s.repo.FindByPersonnelAndPeriod(ctx, p.ID, req.Period)
		if existing != nil && !req.Force {
			result.Skipped = append(result.Skipped, p.FullName)
			continue
		}

		// Attendance stats for the period.
		attendances, _, err := s.attendanceRepo.FindByDateRange(ctx, periodStart, periodEnd, 1, 10000)
		if err != nil {
			return nil, err
		}
		var totalPresent, totalPermission, totalAbsent int
		for _, a := range attendances {
			if a.PersonnelID != p.ID {
				continue
			}
			switch a.Status {
			case constants.AttendanceStatusPresent:
				totalPresent++
			case constants.AttendanceStatusPermission:
				totalPermission++
			case constants.AttendanceStatusAbsent:
				totalAbsent++
			}
		}

		actualWorkingDays := totalPresent + totalPermission
		var attendanceRate float64
		if workingDays > 0 {
			attendanceRate = float64(actualWorkingDays) / float64(workingDays)
		}
		calculatedBase := p.BaseSalary * attendanceRate

		var totalAllowances, totalDeductions float64
		details := make([]model.PayrollDetail, 0, len(components))
		for _, comp := range components {
			if comp.BujpID == nil || *comp.BujpID != req.BujpID {
				continue
			}
			var amount float64
			switch comp.CalculationMethod {
			case constants.SalaryComponentMethodFixed:
				amount = comp.Amount * attendanceRate
			case constants.SalaryComponentMethodPercentage:
				amount = calculatedBase * (comp.Amount / constants.PayrollPercentageDivisor)
			}
			switch comp.Type {
			case constants.SalaryComponentTypeAllowance:
				totalAllowances += amount
			case constants.SalaryComponentTypeDeduction:
				totalDeductions += amount
			}
			details = append(details, model.PayrollDetail{
				SalaryComponentID: comp.ID,
				Amount:            amount,
			})
		}

		netSalary := calculatedBase + totalAllowances - totalDeductions
		payroll := &model.Payroll{
			PersonnelID:     p.ID,
			BujpID:          req.BujpID,
			Period:          req.Period,
			BaseSalary:      calculatedBase,
			TotalAllowances: totalAllowances,
			TotalDeductions: totalDeductions,
			NetSalary:       netSalary,
			TotalPresent:    totalPresent,
			TotalPermission: totalPermission,
			TotalAbsent:     totalAbsent,
			PaymentStatus:   constants.PayrollDefaultPaymentStatus,
			CreatedBy:       createdBy,
		}

		if existing != nil {
			// Force regenerate: drop the old record entirely so details cascade out.
			if err := s.repo.Delete(ctx, existing.ID); err != nil {
				return nil, err
			}
			result.Generated = append(result.Generated, p.FullName+" (updated)")
		} else {
			result.Generated = append(result.Generated, p.FullName)
		}

		if err := s.repo.CreateWithDetails(ctx, payroll, details); err != nil {
			return nil, err
		}
		reloaded, err := s.repo.FindByID(ctx, payroll.ID)
		if err == nil {
			result.Payrolls = append(result.Payrolls, toPayrollResponse(reloaded))
			sendPayrollSlipEmail(p, reloaded)
		}
	}

	return result, nil
}

// sendPayrollSlipEmail dispatches the slip notification in a background
// goroutine so the HTTP handler returns quickly. Errors are logged only;
// payroll generation should never fail because the SMTP server is down.
func sendPayrollSlipEmail(p model.Personnel, payroll *model.Payroll) {
	if p.Email == nil || strings.TrimSpace(*p.Email) == "" || payroll == nil {
		return
	}
	cfg := mailer.LoadConfig()
	if !cfg.IsEnabled() {
		return
	}
	recipient := strings.TrimSpace(*p.Email)
	periodLabel := payroll.Period
	if t, err := time.Parse(constants.PayrollPeriodFormat, payroll.Period); err == nil {
		periodLabel = t.Format("January 2006")
	}
	subject := fmt.Sprintf("Slip Gaji %s - %s", periodLabel, p.FullName)
	html := fmt.Sprintf(`<!doctype html><html><body style="font-family:Arial,sans-serif;color:#1f2937">`+
		`<h2 style="color:#0f172a">Slip Gaji %s</h2>`+
		`<p>Halo <strong>%s</strong>, berikut ringkasan slip gaji Anda untuk periode <strong>%s</strong>.</p>`+
		`<table style="border-collapse:collapse;width:100%%;max-width:520px">`+
		`<tr><td style="padding:6px 0">Gaji Pokok</td><td style="padding:6px 0;text-align:right">Rp %s</td></tr>`+
		`<tr><td style="padding:6px 0">Total Tunjangan</td><td style="padding:6px 0;text-align:right">Rp %s</td></tr>`+
		`<tr><td style="padding:6px 0">Total Potongan</td><td style="padding:6px 0;text-align:right">Rp %s</td></tr>`+
		`<tr><td style="padding:8px 0;border-top:1px solid #e5e7eb;font-weight:600">Gaji Bersih</td>`+
		`<td style="padding:8px 0;border-top:1px solid #e5e7eb;text-align:right;font-weight:600">Rp %s</td></tr>`+
		`</table>`+
		`<p style="margin-top:16px">Kehadiran: %d hadir, %d izin, %d alpa.</p>`+
		`<p style="font-size:12px;color:#6b7280">Email ini dikirim otomatis oleh sistem HRMIS.</p>`+
		`</body></html>`,
		periodLabel,
		p.FullName,
		periodLabel,
		formatRupiah(payroll.BaseSalary),
		formatRupiah(payroll.TotalAllowances),
		formatRupiah(payroll.TotalDeductions),
		formatRupiah(payroll.NetSalary),
		payroll.TotalPresent, payroll.TotalPermission, payroll.TotalAbsent,
	)

	go func(to, subj, body string) {
		if err := mailer.Send(cfg, mailer.Message{To: []string{to}, Subject: subj, HTMLBody: body}); err != nil {
			log.Printf("[payroll] failed to send slip email to %s: %v", to, err)
		}
	}(recipient, subject, html)
}

func formatRupiah(amount float64) string {
	// Format with thousand separators (".") matching id-ID convention.
	whole := int64(amount)
	neg := whole < 0
	if neg {
		whole = -whole
	}
	s := fmt.Sprintf("%d", whole)
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	lead := n % 3
	if lead > 0 {
		b.WriteString(s[:lead])
		if n > lead {
			b.WriteByte('.')
		}
	}
	for i := lead; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteByte('.')
		}
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
