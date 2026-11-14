package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type MonthlyReportHandler struct {
	service  services.MonthlyReportService
	userRepo repository.UserRepository
}

func NewMonthlyReportHandler(service services.MonthlyReportService, userRepo repository.UserRepository) *MonthlyReportHandler {
	return &MonthlyReportHandler{service: service, userRepo: userRepo}
}

// GetAll godoc
// @Summary Get all monthly reports
// @Description Get list of monthly reports with pagination and filters
// @Tags monthly-reports
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param bujp_id query string false "Filter by BUJP ID"
// @Param period query string false "Filter by period (YYYY-MM)"
// @Success 200 {object} model.APIResponse{data=[]model.MonthlyReportResponse}
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/monthly-reports [get]
// @Security BearerAuth
func (h *MonthlyReportHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)

	filters := make(map[string]interface{})
	if bujpID := c.Query("bujp_id"); bujpID != "" {
		filters["bujp_id"] = bujpID
	}
	if period := c.Query("period"); period != "" {
		filters["period"] = period
	}

	utils.ApplyBujpScope(c, filters)

	reports, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	totalPages, hasNext, hasPrevious := utils.CalculatePaginationMeta(page, limit, total)

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved monthly reports",
		Data:    reports,
		Meta: &model.PaginationMeta{
			Page:        page,
			Limit:       limit,
			Total:       total,
			TotalPages:  totalPages,
			HasNext:     hasNext,
			HasPrevious: hasPrevious,
		},
	})
}

// GetByID godoc
// @Summary Get monthly report by ID
// @Description Get monthly report details by ID
// @Tags monthly-reports
// @Produce json
// @Param id path string true "Monthly Report ID"
// @Success 200 {object} model.APIResponse{data=model.MonthlyReportResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/monthly-reports/{id} [get]
// @Security BearerAuth
func (h *MonthlyReportHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Invalid UUID format",
		})
	}

	if handled, err := h.authorize(c, ctx, id); handled {
		return err
	}

	report, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved monthly report",
		Data:    report,
	})
}

// Create godoc
// @Summary Create new monthly report
// @Description Create a new monthly report
// @Tags monthly-reports
// @Accept json
// @Produce json
// @Param request body model.CreateMonthlyReportRequest true "Monthly report data"
// @Success 201 {object} model.APIResponse{data=model.MonthlyReportResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/monthly-reports [post]
// @Security BearerAuth
func (h *MonthlyReportHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req model.CreateMonthlyReportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Get user ID from context (from auth middleware)
	var createdBy *uuid.UUID
	if userID := c.Locals("userID"); userID != nil {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	company, ok := ownCompany(c, req.BujpID)
	if !ok {
		return forbiddenResponse(c)
	}
	req.BujpID = company

	report, err := h.service.Create(ctx, &req, createdBy)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: "Successfully created monthly report",
		Data:    report,
	})
}

// Update godoc
// @Summary Update monthly report
// @Description Update monthly report by ID
// @Tags monthly-reports
// @Accept json
// @Produce json
// @Param id path string true "Monthly Report ID"
// @Param request body model.UpdateMonthlyReportRequest true "Monthly report data"
// @Success 200 {object} model.APIResponse{data=model.MonthlyReportResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/monthly-reports/{id} [put]
// @Security BearerAuth
func (h *MonthlyReportHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Invalid UUID format",
		})
	}

	var req model.UpdateMonthlyReportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	if handled, err := h.authorize(c, ctx, id); handled {
		return err
	}

	report, err := h.service.Update(ctx, id, &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully updated monthly report",
		Data:    report,
	})
}

// Delete godoc
// @Summary Delete monthly report
// @Description Delete monthly report by ID
// @Tags monthly-reports
// @Produce json
// @Param id path string true "Monthly Report ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/monthly-reports/{id} [delete]
// @Security BearerAuth
func (h *MonthlyReportHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Invalid UUID format",
		})
	}

	if handled, err := h.authorize(c, ctx, id); handled {
		return err
	}

	if err := h.service.Delete(ctx, id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully deleted monthly report",
	})
}

// Generate godoc
// @Summary Generate monthly report
// @Description Calculate aggregated stats for a (BUJP, period) and persist a monthly report. Non-superadmin callers are clamped to their own BUJP.
// @Tags monthly-reports
// @Accept json
// @Produce json
// @Param request body model.GenerateMonthlyReportRequest true "Generate request"
// @Success 200 {object} model.APIResponse{data=model.MonthlyReportResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/monthly-reports/generate [post]
// @Security BearerAuth
func (h *MonthlyReportHandler) Generate(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var req model.GenerateMonthlyReportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Resolve caller for BUJP scoping.
	var createdBy *uuid.UUID
	role, _ := c.Locals("role").(string)
	if userID := c.Locals("userID"); userID != nil {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
			// Non-superadmin/admin must operate within their own BUJP.
			if role != "super_admin" && role != "admin" {
				caller, err := h.userRepo.FindByID(ctx, uid)
				if err != nil || caller == nil || caller.BujpID == nil {
					return c.Status(http.StatusForbidden).JSON(model.APIResponse{
						Status:  "error",
						Code:    http.StatusForbidden,
						Message: "Akun Anda tidak terhubung ke BUJP manapun",
					})
				}
				req.BujpID = *caller.BujpID
			}
		}
	}

	if req.BujpID == uuid.Nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "bujp_id wajib diisi",
		})
	}

	company, ok := ownCompany(c, req.BujpID)
	if !ok || company == uuid.Nil {
		return forbiddenResponse(c)
	}
	req.BujpID = company

	report, err := h.service.Generate(ctx, req.BujpID, req.Period, req.Force, createdBy)
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		switch {
		case strings.Contains(msg, "sudah ada"), strings.Contains(msg, "already exists"):
			status = http.StatusConflict
		case strings.Contains(msg, "wajib"), strings.Contains(msg, "format"), strings.Contains(msg, "tidak valid"):
			status = http.StatusBadRequest
		}
		return c.Status(status).JSON(model.APIResponse{
			Status:  "error",
			Code:    status,
			Message: msg,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Laporan bulanan berhasil dibuat",
		Data:    report,
	})
}

// authorize loads the report and checks it belongs to the caller's company.
func (h *MonthlyReportHandler) authorize(c *fiber.Ctx, ctx context.Context, id uuid.UUID) (bool, error) {
	rec, err := h.service.GetByID(ctx, id)
	if err != nil || rec == nil {
		return true, notFoundResponse(c, "Monthly report")
	}
	if !utils.CanAccessBujp(c, rec.BujpID) {
		return true, forbiddenResponse(c)
	}
	return false, nil
}

// recapScope resolves the company, office and person of a recap request. The company comes from the
// session; central roles must name one. An office or person outside that company is refused.
func (h *MonthlyReportHandler) recapScope(c *fiber.Ctx, ctx context.Context) (company, location, person uuid.UUID, denied error, handled bool) {
	company = utils.CallerBujpID(c)
	if utils.IsPusat(utils.CallerRole(c)) {
		company = uuid.Nil
		if v := c.Query("bujp_id"); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return uuid.Nil, uuid.Nil, uuid.Nil, c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id is invalid"}), true
			}
			company = id
		}
	}
	if company == uuid.Nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id wajib diisi"}), true
	}
	if v := c.Query("location_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil || !locationAllowed(c, ctx, id) {
			return uuid.Nil, uuid.Nil, uuid.Nil, forbiddenResponse(c), true
		}
		location = id
	}
	if v := c.Query("personnel_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil || !personAllowed(c, ctx, id, nil) {
			return uuid.Nil, uuid.Nil, uuid.Nil, forbiddenResponse(c), true
		}
		person = id
	}
	return company, location, person, nil, false
}

// Recap returns the per-person recap of a month: present, late, absent, leave and overtime.
//
// @Router /monthly-reports/recap [get]
func (h *MonthlyReportHandler) Recap(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	company, location, person, err, handled := h.recapScope(c, ctx)
	if handled {
		return err
	}
	recap, rerr := h.service.Recap(ctx, company, c.Query("period"), location, person, c.Query("search"))
	if rerr != nil {
		status := http.StatusInternalServerError
		if strings.Contains(rerr.Error(), "period") || strings.Contains(rerr.Error(), "required") {
			status = http.StatusBadRequest
		}
		return c.Status(status).JSON(model.APIResponse{Status: "error", Code: status, Message: rerr.Error()})
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Rekap absensi bulanan", Data: recap})
}

// RecapExport is the recap as an Excel file.
//
// @Router /monthly-reports/recap/export [get]
func (h *MonthlyReportHandler) RecapExport(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	company, location, person, err, handled := h.recapScope(c, ctx)
	if handled {
		return err
	}
	recap, rerr := h.service.Recap(ctx, company, c.Query("period"), location, person, c.Query("search"))
	if rerr != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: rerr.Error()})
	}
	data, werr := recapWorkbook(recap)
	if werr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Gagal membuat file Excel"})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="rekap-absensi-`+recap.Period+`.xlsx"`)
	return c.Send(data)
}

// recapWorkbook renders the recap into an .xlsx (header, one row per person, totals).
func recapWorkbook(r *model.AttendanceRecap) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Rekap " + r.Period
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Nama", "NIK", "NIP", "Hadir", "Telat", "Tidak Hadir", "Izin/Cuti/Sakit", "Lembur (jam)", "Jam Kerja (jam)"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	row := func(n int, name, nik, nip string, x model.AttendanceRecapRow) {
		vals := []interface{}{name, nik, nip, x.Hadir, x.Telat, x.TidakHadir, x.Izin, x.LemburJam, float64(x.JamKerjaMenit) / 60}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, n)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	for i, x := range r.Rows {
		nip := ""
		if x.NIP != nil {
			nip = *x.NIP
		}
		row(i+2, spreadsheetSafe(x.FullName), spreadsheetSafe(x.IDNumber), spreadsheetSafe(nip), x)
	}
	row(len(r.Rows)+2, "TOTAL", "", "", r.Totals)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// spreadsheetSafe stops a name that starts with = + - @ from being read as a formula.
func spreadsheetSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}
