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
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
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
	correctionRepo      repository.AttendanceCorrectionRepository
	salaryComponentRepo repository.SalaryComponentRepository
	notifier            *notifier.Notifier
}

func NewPayrollService(
	repo repository.PayrollRepository,
	personnelRepo repository.PersonnelRepository,
	attendanceRepo repository.AttendanceRepository,
	correctionRepo repository.AttendanceCorrectionRepository,
	salaryComponentRepo repository.SalaryComponentRepository,
	notif *notifier.Notifier,
) PayrollService {
	return &payrollServiceImpl{
		repo:                repo,
		personnelRepo:       personnelRepo,
		attendanceRepo:      attendanceRepo,
		correctionRepo:      correctionRepo,
		salaryComponentRepo: salaryComponentRepo,
		notifier:            notif,
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

type payrollAttendanceStats struct {
	present          int
	permission       int
	absent           int
	correctedPresent int
}

func isWorkingDay(d time.Time) bool {
	w := d.Weekday()
	return w != time.Saturday && w != time.Sunday
}

func countWorkingDays(start, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	n := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if isWorkingDay(d) {
			n++
		}
	}
	return n
}

func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

func classifyAttendanceStatus(status string) string {
	switch status {
	case constants.AttendanceStatusPresent, "late":
		return "present"
	case constants.AttendanceStatusPermission,
		constants.AttendanceStatusSick,
		constants.AttendanceStatusLeave:
		return "permission"
	case constants.AttendanceStatusAbsent:
		return "absent"
	}
	return ""
}

func computeAttendanceStats(
	periodStart, evalEnd time.Time,
	attendances []model.Attendance,
	corrections []model.AttendanceCorrection,
) payrollAttendanceStats {
	attendanceByDate := make(map[string]string, len(attendances))
	for _, a := range attendances {
		attendanceByDate[dateKey(a.Date)] = classifyAttendanceStatus(a.Status)
	}
	correctionDates := make(map[string]bool, len(corrections))
	for _, c := range corrections {
		correctionDates[dateKey(c.CorrectionDate)] = true
	}

	var stats payrollAttendanceStats
	if evalEnd.Before(periodStart) {
		return stats
	}
	for d := periodStart; !d.After(evalEnd); d = d.AddDate(0, 0, 1) {
		if !isWorkingDay(d) {
			continue
		}
		key := dateKey(d)
		if bucket, ok := attendanceByDate[key]; ok {
			switch bucket {
			case "present":
				stats.present++
			case "permission":
				stats.permission++
			default:
				// absent or unknown
				if correctionDates[key] {
					stats.present++
					stats.correctedPresent++
				} else {
					stats.absent++
				}
			}
			continue
		}
		if correctionDates[key] {
			stats.present++
			stats.correctedPresent++
			continue
		}
		stats.absent++
	}
	return stats
}

func computeSalaryComponents(
	baseSalary float64,
	bujpID uuid.UUID,
	components []model.SalaryComponent,
) (totalAllowances, totalDeductions float64, details []model.PayrollDetail) {
	details = make([]model.PayrollDetail, 0, len(components))
	for _, comp := range components {
		if comp.BujpID == nil || *comp.BujpID != bujpID {
			continue
		}
		var amount float64
		switch comp.CalculationMethod {
		case constants.SalaryComponentMethodFixed:
			amount = comp.Amount
		case constants.SalaryComponentMethodPercentage:
			amount = baseSalary * (comp.Amount / constants.PayrollPercentageDivisor)
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
	return
}

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

	workingDays := countWorkingDays(periodStart, periodEnd)

	evalEnd := periodEnd
	if today := time.Now(); today.Before(evalEnd) {
		evalEnd = today
	}
	expectedWorkingDays := countWorkingDays(periodStart, evalEnd)

	// Active salary components for this BUJP.
	components, _, err := s.salaryComponentRepo.FindAll(ctx, 1, 1000, map[string]interface{}{
		"bujp_id":   req.BujpID,
		"is_active": true,
	})
	if err != nil {
		return nil, err
	}

	allAttendances, _, err := s.attendanceRepo.FindByDateRange(ctx, periodStart, periodEnd, 1, 100000)
	if err != nil {
		return nil, err
	}
	attendanceByPersonnel := make(map[uuid.UUID][]model.Attendance, len(personnels))
	for _, a := range allAttendances {
		attendanceByPersonnel[a.PersonnelID] = append(attendanceByPersonnel[a.PersonnelID], a)
	}

	correctionsByPersonnel := make(map[uuid.UUID][]model.AttendanceCorrection, len(personnels))
	if s.correctionRepo != nil {
		approvedCorrections, cerr := s.correctionRepo.FindApprovedByDateRange(ctx, req.BujpID, periodStart, periodEnd)
		if cerr != nil {
			return nil, cerr
		}
		for _, c := range approvedCorrections {
			correctionsByPersonnel[c.PersonnelID] = append(correctionsByPersonnel[c.PersonnelID], c)
		}
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

		stats := computeAttendanceStats(
			periodStart, evalEnd,
			attendanceByPersonnel[p.ID],
			correctionsByPersonnel[p.ID],
		)

		baseSalary := p.BaseSalary
		var dailyRate float64
		if workingDays > 0 {
			dailyRate = baseSalary / float64(workingDays)
		}
		absenceDeduction := dailyRate * float64(stats.absent)

		totalAllowances, totalDeductions, details := computeSalaryComponents(baseSalary, req.BujpID, components)
		totalDeductions += absenceDeduction

		netSalary := baseSalary + totalAllowances - totalDeductions
		payroll := &model.Payroll{
			PersonnelID:     p.ID,
			BujpID:          req.BujpID,
			Period:          req.Period,
			BaseSalary:      baseSalary,
			TotalAllowances: totalAllowances,
			TotalDeductions: totalDeductions,
			NetSalary:       netSalary,
			TotalPresent:    stats.present,
			TotalPermission: stats.permission,
			TotalAbsent:     stats.absent,
			PaymentStatus:   constants.PayrollDefaultPaymentStatus,
			CreatedBy:       createdBy,
		}

		if existing != nil {
			// Force regenerate: drop old record so details cascade out.
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
			s.sendPayrollSlipEmail(ctx, p, reloaded, workingDays, expectedWorkingDays, stats, dailyRate, absenceDeduction)
		}
	}

	return result, nil
}

func (s *payrollServiceImpl) sendPayrollSlipEmail(ctx context.Context, p model.Personnel, payroll *model.Payroll, workingDays, expectedWorkingDays int, stats payrollAttendanceStats, dailyRate, absenceDeduction float64) {
	if p.Email == nil || strings.TrimSpace(*p.Email) == "" || payroll == nil {
		return
	}
	recipient := strings.TrimSpace(*p.Email)
	periodLabel := payroll.Period
	if t, err := time.Parse(constants.PayrollPeriodFormat, payroll.Period); err == nil {
		periodLabel = formatPeriodID(t)
	}
	subject := fmt.Sprintf("[SAKU] Slip Gaji %s \u2014 %s", periodLabel, p.FullName)
	html := buildPayrollSlipHTML(p, payroll, periodLabel, workingDays, expectedWorkingDays, stats, dailyRate, absenceDeduction)

	if s.notifier != nil {
		s.notifier.Send(ctx, []string{recipient}, subject, html)
		return
	}
	cfg := mailer.LoadConfig()
	if !cfg.IsEnabled() {
		return
	}
	if err := mailer.Send(cfg, mailer.Message{To: []string{recipient}, Subject: subject, HTMLBody: html}); err != nil {
		log.Printf("[payroll] fallback slip email failed for %s: %v", recipient, err)
	}
}

func buildPayrollSlipHTML(p model.Personnel, payroll *model.Payroll, periodLabel string, workingDays, expectedWorkingDays int, stats payrollAttendanceStats, dailyRate, absenceDeduction float64) string {
	initials := buildInitials(p.FullName)
	bujpName := "-"
	if payroll.Bujp != nil {
		bujpName = payroll.Bujp.Name
	}
	email := "-"
	if p.Email != nil && strings.TrimSpace(*p.Email) != "" {
		email = strings.TrimSpace(*p.Email)
	}
	bankInfo := "-"
	bank := strPtrOrDash(p.BankName)
	acc := strPtrOrDash(p.AccountNumber)
	if bank != "-" || acc != "-" {
		bankInfo = bank + " &ndash; " + acc
	}
	idNumber := p.IDNumber
	if idNumber == "" {
		idNumber = "-"
	}

	var allowanceRows, deductionRows strings.Builder
	for _, d := range payroll.PayrollDetails {
		if d.SalaryComponent == nil {
			continue
		}
		row := fmt.Sprintf(`<tr><td style="padding:13px 16px;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;color:#334155">%s</p></td><td style="padding:13px 16px;text-align:right;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;font-weight:500;color:%s;font-family:'SF Mono','Cascadia Code','Courier New',monospace">%s Rp %s</p></td></tr>`,
			d.SalaryComponent.Name,
			"%COLOR%",
			"%SIGN%",
			formatRupiah(d.Amount),
		)
		switch d.SalaryComponent.Type {
		case constants.SalaryComponentTypeAllowance:
			allowanceRows.WriteString(strings.NewReplacer("%COLOR%", "#16a34a", "%SIGN%", "+").Replace(row))
		case constants.SalaryComponentTypeDeduction:
			deductionRows.WriteString(strings.NewReplacer("%COLOR%", "#dc2626", "%SIGN%", "-").Replace(row))
		}
	}

	if absenceDeduction > 0 {
		deductionRows.WriteString(fmt.Sprintf(`<tr><td style="padding:13px 16px;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;color:#334155">Potongan Absen <span style="color:#94a3b8;font-size:12px">(%d hari × Rp %s)</span></p></td><td style="padding:13px 16px;text-align:right;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;font-weight:500;color:#dc2626;font-family:'SF Mono','Cascadia Code','Courier New',monospace">- Rp %s</p></td></tr>`,
			stats.absent, formatRupiah(dailyRate), formatRupiah(absenceDeduction)))
	}

	deductionSection := ""
	if deductionRows.Len() > 0 {
		deductionSection = `<tr><td colspan="2" style="padding:9px 16px;background-color:#f8fafc;border-top:1px solid #e2e8f0;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:0.6px">Potongan</p></td></tr>` + deductionRows.String()
	}

	correctionRow := ""
	if stats.correctedPresent > 0 {
		correctionRow = fmt.Sprintf(`<tr><td style="padding:11px 16px;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;color:#64748b">Hari Hadir dari Koreksi Absen</p></td><td style="padding:11px 16px;text-align:right;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;font-weight:600;color:#16a34a;font-family:'SF Mono','Cascadia Code','Courier New',monospace">+%d hari</p></td></tr>`, stats.correctedPresent)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>Slip Gaji %s</title>
<style>@media only screen and (max-width:620px){.email-wrapper{width:100%%!important}.body-pad{padding:24px 20px!important}.header-pad{padding:24px 20px!important}.att-value{font-size:22px!important}.summary-stack td{display:block!important;width:100%%!important;padding:10px 16px!important;border-right:none!important;border-bottom:1px solid #e2e8f0!important;border-radius:0!important}.summary-stack td:last-child{border-bottom:none!important}.hero-amount{font-size:30px!important}}</style>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:'Aptos','Aptos Display',-apple-system,BlinkMacSystemFont,'SF Pro Display','SF Pro Text','Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale">
<div style="display:none;max-height:0;overflow:hidden;font-size:1px;line-height:1px;color:#f1f5f9">Gaji bersih Rp %s untuk periode %s telah diterbitkan.</div>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f1f5f9">
<tr><td align="center" style="padding:32px 16px">
<table role="presentation" class="email-wrapper" width="600" cellpadding="0" cellspacing="0" style="width:600px;max-width:100%%;background-color:#ffffff;border-radius:18px;overflow:hidden;box-shadow:0 1px 2px rgba(15,23,42,0.04),0 8px 24px rgba(15,23,42,0.06);border:1px solid #e2e8f0">

<!-- Brand strip -->
<tr><td style="background:linear-gradient(135deg,#10b981 0%%,#059669 100%%);padding:6px 0"></td></tr>

<!-- HEADER -->
<tr><td style="padding:30px 36px 24px;border-bottom:1px solid #f1f5f9" class="header-pad">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr>
<td><p style="margin:0 0 8px;font-size:11px;font-weight:600;color:#94a3b8;letter-spacing:1.4px;text-transform:uppercase">SAKU &middot; Aplikasi Kepegawaian</p>
<h1 style="margin:0 0 6px;font-size:24px;font-weight:700;color:#0f172a;line-height:1.2;letter-spacing:-0.4px">Slip Gaji</h1>
<p style="margin:0;font-size:13.5px;color:#64748b">Periode <strong style="color:#334155">%s</strong></p></td>
<td align="right" valign="top"><span style="display:inline-block;background-color:#ecfdf5;color:#047857;font-size:11px;font-weight:600;padding:6px 14px;border-radius:999px;border:1px solid #a7f3d0;white-space:nowrap;letter-spacing:0.3px">Diterbitkan</span></td>
</tr></table></td></tr>

<!-- BODY -->
<tr><td style="padding:28px 36px 8px" class="body-pad">

<!-- Hero net pay -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:linear-gradient(135deg,#ecfdf5 0%%,#d1fae5 100%%);border:1px solid #a7f3d0;border-radius:14px;margin:0 0 24px"><tr><td style="padding:24px 24px;text-align:center">
<p style="margin:0 0 6px;font-size:11px;font-weight:600;color:#047857;text-transform:uppercase;letter-spacing:1px">Gaji Bersih Diterima</p>
<p style="margin:0;font-size:34px;font-weight:700;color:#065f46;font-family:'Aptos Display','Aptos','SF Pro Display',-apple-system,BlinkMacSystemFont,sans-serif;letter-spacing:-0.8px" class="hero-amount">Rp %s</p>
<p style="margin:8px 0 0;font-size:12.5px;color:#047857">Akan ditransfer ke rekening Anda yang terdaftar</p>
</td></tr></table>

<!-- Employee -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:26px"><tr><td>
<table role="presentation" cellpadding="0" cellspacing="0" style="margin-bottom:14px"><tr>
<td style="width:48px;height:48px;border-radius:50%%;background-color:#ecfdf5;vertical-align:middle" width="48"><p style="margin:0;font-size:16px;font-weight:700;color:#059669;text-align:center;line-height:48px">%s</p></td>
<td style="padding-left:14px;vertical-align:middle"><p style="margin:0 0 3px;font-size:16px;font-weight:700;color:#0f172a;letter-spacing:-0.2px">%s</p><p style="margin:0;font-size:13px;color:#64748b">NIK %s</p></td>
</tr></table>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:12px;overflow:hidden">
<tr>
<td style="padding:12px 16px;border-right:1px solid #e2e8f0;border-bottom:1px solid #e2e8f0;width:50%%"><p style="margin:0 0 3px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.6px">Perusahaan</p><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b">%s</p></td>
<td style="padding:12px 16px;border-bottom:1px solid #e2e8f0;width:50%%"><p style="margin:0 0 3px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.6px">Email</p><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b">%s</p></td>
</tr><tr>
<td style="padding:12px 16px;border-right:1px solid #e2e8f0"><p style="margin:0 0 3px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.6px">Rekening</p><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b">%s</p></td>
<td style="padding:12px 16px"><p style="margin:0 0 3px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.6px">Periode</p><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b">%s</p></td>
</tr></table></td></tr></table>

<!-- Attendance -->
<p style="margin:0 0 12px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.8px">Ringkasan Kehadiran</p>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:26px"><tr>
<td style="width:33.33%%;padding-right:5px"><table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f0fdf4;border-radius:12px;border:1px solid #bbf7d0"><tr><td style="padding:16px 12px;text-align:center"><p style="margin:0 0 6px;font-size:10px;font-weight:600;color:#166534;text-transform:uppercase;letter-spacing:0.5px">Hadir</p><p style="margin:0 0 3px;font-size:26px;font-weight:700;color:#16a34a" class="att-value">%d</p><p style="margin:0;font-size:10px;color:#4ade80">hari</p></td></tr></table></td>
<td style="width:33.33%%;padding:0 3px"><table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#fefce8;border-radius:12px;border:1px solid #fde68a"><tr><td style="padding:16px 12px;text-align:center"><p style="margin:0 0 6px;font-size:10px;font-weight:600;color:#854d0e;text-transform:uppercase;letter-spacing:0.5px">Izin</p><p style="margin:0 0 3px;font-size:26px;font-weight:700;color:#ca8a04" class="att-value">%d</p><p style="margin:0;font-size:10px;color:#fbbf24">hari</p></td></tr></table></td>
<td style="width:33.33%%;padding-left:5px"><table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#fef2f2;border-radius:12px;border:1px solid #fecaca"><tr><td style="padding:16px 12px;text-align:center"><p style="margin:0 0 6px;font-size:10px;font-weight:600;color:#991b1b;text-transform:uppercase;letter-spacing:0.5px">Absen</p><p style="margin:0 0 3px;font-size:26px;font-weight:700;color:#ef4444" class="att-value">%d</p><p style="margin:0;font-size:10px;color:#f87171">hari</p></td></tr></table></td>
</tr></table>

<!-- Pro-rate breakdown -->
<p style="margin:0 0 12px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.8px">Perhitungan Kehadiran</p>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:12px;overflow:hidden;margin-bottom:26px;background-color:#fafbfc">
<tr><td style="padding:11px 16px;border-bottom:1px solid #e2e8f0;width:55%%"><p style="margin:0;font-size:13px;color:#64748b">Total Hari Kerja Periode</p></td><td style="padding:11px 16px;text-align:right;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b;font-family:'SF Mono','Cascadia Code','Courier New',monospace">%d hari</p></td></tr>
<tr><td style="padding:11px 16px;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;color:#64748b">Hari Kerja Diharapkan (s/d hari ini)</p></td><td style="padding:11px 16px;text-align:right;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b;font-family:'SF Mono','Cascadia Code','Courier New',monospace">%d hari</p></td></tr>
%s
<tr><td style="padding:11px 16px;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;color:#64748b">Tarif Harian (Gaji / Hari Kerja)</p></td><td style="padding:11px 16px;text-align:right;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:13px;font-weight:500;color:#1e293b;font-family:'SF Mono','Cascadia Code','Courier New',monospace">Rp %s</p></td></tr>
<tr><td style="padding:11px 16px"><p style="margin:0;font-size:13px;color:#64748b">Hari Alpa (otomatis dipotong)</p></td><td style="padding:11px 16px;text-align:right"><p style="margin:0;font-size:13px;font-weight:600;color:#dc2626;font-family:'SF Mono','Cascadia Code','Courier New',monospace">%d hari</p></td></tr>
</table>

<!-- Salary Detail -->
<p style="margin:0 0 12px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.8px">Rincian Gaji</p>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:12px;overflow:hidden;margin-bottom:24px">
<tr><td colspan="2" style="padding:9px 16px;background-color:#fafbfc;border-bottom:1px solid #e2e8f0"><p style="margin:0;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:0.6px">Pendapatan</p></td></tr>
<tr><td style="padding:13px 16px;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;color:#334155">Gaji Pokok</p></td><td style="padding:13px 16px;text-align:right;border-bottom:1px solid #f1f5f9"><p style="margin:0;font-size:14px;font-weight:500;color:#1e293b;font-family:'SF Mono','Cascadia Code','Courier New',monospace">Rp %s</p></td></tr>
%s
%s
<tr><td style="padding:16px;background-color:#ecfdf5;border-top:1px solid #a7f3d0"><p style="margin:0;font-size:15px;font-weight:700;color:#065f46;letter-spacing:-0.2px">Total Gaji Bersih</p></td><td style="padding:16px;text-align:right;background-color:#ecfdf5;border-top:1px solid #a7f3d0"><p style="margin:0;font-size:18px;font-weight:700;color:#065f46;font-family:'SF Mono','Cascadia Code','Courier New',monospace;letter-spacing:-0.3px">Rp %s</p></td></tr>
</table>

<!-- Summary -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:18px"><tr class="summary-stack">
<td style="width:33.33%%;padding:14px 16px;background-color:#fafbfc;border-radius:12px 0 0 12px;border:1px solid #e2e8f0;border-right:none"><p style="margin:0 0 5px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.5px">Total Pendapatan</p><p style="margin:0;font-size:13px;font-weight:600;color:#1e293b;font-family:'SF Mono','Cascadia Code','Courier New',monospace">Rp %s</p></td>
<td style="width:33.33%%;padding:14px 16px;background-color:#fafbfc;border:1px solid #e2e8f0;border-right:none"><p style="margin:0 0 5px;font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.5px">Total Potongan</p><p style="margin:0;font-size:13px;font-weight:600;color:#dc2626;font-family:'SF Mono','Cascadia Code','Courier New',monospace">Rp %s</p></td>
<td style="width:33.33%%;padding:14px 16px;background-color:#f0fdf4;border-radius:0 12px 12px 0;border:1px solid #bbf7d0"><p style="margin:0 0 5px;font-size:10px;font-weight:600;color:#166534;text-transform:uppercase;letter-spacing:0.5px">Dibawa Pulang</p><p style="margin:0;font-size:13px;font-weight:600;color:#16a34a;font-family:'SF Mono','Cascadia Code','Courier New',monospace">Rp %s</p></td>
</tr></table>

<!-- Thank you note -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#eff6ff;border:1px solid #bfdbfe;border-radius:12px;margin:0 0 8px"><tr><td style="padding:14px 18px">
<p style="margin:0 0 4px;font-size:13px;font-weight:700;color:#1e40af">Terima kasih atas dedikasi Anda &#128170;</p>
<p style="margin:0;font-size:13px;color:#1e40af;line-height:1.6">Slip ini bersifat resmi dan dapat digunakan sebagai bukti pembayaran. Jika ada pertanyaan terkait perhitungan, silakan hubungi bagian HRD perusahaan Anda.</p>
</td></tr></table>

</td></tr>

<!-- FOOTER -->
<tr><td style="padding:22px 36px 26px;border-top:1px solid #f1f5f9;background-color:#fafbfc;text-align:center">
<p style="margin:0 0 6px;font-size:12px;color:#94a3b8;line-height:1.6">Slip gaji ini dibuat secara otomatis oleh sistem SAKU.</p>
<p style="margin:0 0 4px;font-size:11.5px;color:#94a3b8">Untuk pertanyaan terkait gaji, silakan hubungi bagian HRD.</p>
<p style="margin:14px 0 0;font-size:11px;color:#cbd5e1;letter-spacing:0.3px">&copy; %d SAKU &bull; Aplikasi Kepegawaian</p>
</td></tr>

</table>
</td></tr></table>
</body></html>`,
		periodLabel,
		formatRupiah(payroll.NetSalary), periodLabel,
		periodLabel,
		formatRupiah(payroll.NetSalary),
		initials,
		p.FullName,
		idNumber,
		bujpName,
		email,
		bankInfo,
		periodLabel,
		payroll.TotalPresent, payroll.TotalPermission, payroll.TotalAbsent,
		workingDays,
		expectedWorkingDays,
		correctionRow,
		formatRupiah(dailyRate),
		stats.absent,
		formatRupiah(payroll.BaseSalary),
		allowanceRows.String(),
		deductionSection,
		formatRupiah(payroll.NetSalary),
		formatRupiah(payroll.BaseSalary+payroll.TotalAllowances),
		formatRupiah(payroll.TotalDeductions),
		formatRupiah(payroll.NetSalary),
		time.Now().Year(),
	)
}

func formatPeriodID(t time.Time) string {
	months := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	return fmt.Sprintf("%s %d", months[int(t.Month())-1], t.Year())
}

func buildInitials(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "?"
	}
	out := strings.ToUpper(string(parts[0][0]))
	if len(parts) > 1 {
		out += strings.ToUpper(string(parts[1][0]))
	}
	return out
}

func strPtrOrDash(s *string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return "-"
	}
	return strings.TrimSpace(*s)
}

func formatRupiah(amount float64) string {
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
