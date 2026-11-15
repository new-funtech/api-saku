package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type LoanHandler struct {
	service services.LoanService
}

func NewLoanHandler(service services.LoanService) *LoanHandler {
	return &LoanHandler{service: service}
}

// GetAll godoc
// @Summary      List loans
// @Tags         Loans
// @Produce      json
// @Param        page            query int    false "Page"
// @Param        limit           query int    false "Limit"
// @Param        status          query string false "Filter by status"
// @Param        bujp_id         query string false "Filter by BUJP"
// @Param        personnel_id    query string false "Filter by Personnel"
// @Param        loan_product_id query string false "Filter by Loan Product"
// @Param        search          query string false "Search by loan number"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans [get]
func (h *LoanHandler) GetAll(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)
	filters := map[string]interface{}{}
	if v := c.Query("status"); v != "" {
		filters["status"] = v
	} else if v := c.Query("status_in"); v != "" {
		filters["status_in"] = strings.Split(v, ",")
	}
	if v := c.Query("search"); v != "" {
		filters["search"] = v
	}
	if v := c.Query("bujp_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filters["bujp_id"] = id
		}
	}
	if v := c.Query("personnel_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filters["personnel_id"] = id
		}
	}
	if v := c.Query("loan_product_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filters["loan_product_id"] = id
		}
	}

	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	switch role {
	case "company_admin", "supervisor":
		if bid := h.service.ResolveUserBujpID(c.Context(), uid); bid != uuid.Nil {
			filters["bujp_id"] = bid
		} else {
			filters["force_empty"] = true
		}
	case "guard":
		if pid := h.service.ResolveUserPersonnelID(c.Context(), uid); pid != uuid.Nil {
			filters["personnel_id"] = pid
		} else {
			filters["force_empty"] = true
		}
	case "super_admin", "admin", "bprks":
		_, hasStatus := filters["status"]
		_, hasStatusIn := filters["status_in"]
		if !hasStatus && !hasStatusIn {
			filters["status_in"] = []string{
				model.LoanStatusApprovedBujp,
				model.LoanStatusApprovedPusat,
				model.LoanStatusPendingUserConfirmation,
				model.LoanStatusDisbursed,
				model.LoanStatusActive,
				model.LoanStatusCompleted,
				model.LoanStatusRejected,
				model.LoanStatusCancelled,
			}
		}
	}

	items, total, err := h.service.GetAll(c.Context(), page, limit, filters)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "loans retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

// GetByID godoc
// @Summary      Get loan by ID
// @Tags         Loans
// @Produce      json
// @Param        id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id} [get]
func (h *LoanHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	r, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusNotFound, err.Error())
	}

	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	switch role {
	case "company_admin", "supervisor":
		bid := h.service.ResolveUserBujpID(c.Context(), uid)
		if bid == uuid.Nil || r.BujpID == nil || *r.BujpID != bid {
			return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
		}
	case "guard":
		pid := h.service.ResolveUserPersonnelID(c.Context(), uid)
		if pid == uuid.Nil || r.PersonnelID != pid {
			return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
		}
	}

	return utils.SuccessResponse(c, http.StatusOK, "loan retrieved", r)
}

// History godoc
// @Summary      Get loan history for a personnel
// @Description  If personnel_id is omitted, returns history for the authenticated user's personnel.
// @Tags         Loans
// @Produce      json
// @Param        personnel_id path string false "Personnel UUID (optional)"
// @Param        page  query int false "Page"
// @Param        limit query int false "Limit"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/history/{personnel_id} [get]
func (h *LoanHandler) History(c *fiber.Ctx) error {
	var personnelID uuid.UUID
	if raw := c.Params("personnel_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return utils.ErrorResponse(c, http.StatusBadRequest, "invalid personnel id")
		}
		personnelID = id
	}
	if personnelID != uuid.Nil && utils.CallerRole(c) != "bprks" && !personAllowed(c, c.Context(), personnelID, nil) {
		return forbiddenResponse(c)
	}
	page, limit := utils.ParsePagination(c)

	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	if personnelID == uuid.Nil && role != "guard" {
		filters := loanHistoryStatusFilters(c)
		if v := c.Query("bujp_id"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				filters["bujp_id"] = id
			}
		}
		if v := c.Query("personnel_id"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				filters["personnel_id"] = id
			}
		}

		utils.ApplyBujpScope(c, filters)

		items, total, err := h.service.GetAll(c.Context(), page, limit, filters)
		if err != nil {
			return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		}
		totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
		return utils.SuccessResponseWithMeta(c, http.StatusOK, "loan history retrieved", items, &model.PaginationMeta{
			Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
		})
	}

	if personnelID == uuid.Nil {
		if uid == uuid.Nil {
			return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		}
		resolved, err := h.service.GetMyHistoryByUser(c.Context(), uid, page, limit, loanHistoryStatusFilters(c))
		if err != nil {
			return utils.ErrorResponse(c, statusForServiceError(err), err.Error())
		}
		totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, resolved.Total)
		return utils.SuccessResponseWithMeta(c, http.StatusOK, "loan history retrieved", resolved.Items, &model.PaginationMeta{
			Page: page, Limit: limit, Total: resolved.Total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
		})
	}

	items, total, err := h.service.GetMyHistory(c.Context(), personnelID, page, limit, loanHistoryStatusFilters(c))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "loan history retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

func loanHistoryStatusFilters(c *fiber.Ctx) map[string]interface{} {
	filters := make(map[string]interface{})
	status := c.Query("status", "")
	switch status {
	case "", "all":
		filters["status_in"] = []string{
			model.LoanStatusApprovedBujp,
			model.LoanStatusApprovedPusat,
			model.LoanStatusPendingUserConfirmation,
			model.LoanStatusDisbursed,
			model.LoanStatusActive,
			model.LoanStatusCompleted,
			model.LoanStatusRejected,
			model.LoanStatusCancelled,
		}
	default:
		filters["status"] = status
	}
	if v := c.Query("search"); v != "" {
		filters["search"] = v
	}
	return filters
}

// Create godoc
// @Summary      Create a loan (draft or submitted)
// @Tags         Loans
// @Accept       multipart/form-data
// @Accept       json
// @Produce      json
// @Param        body body model.CreateLoanRequest true "Body"
// @Success      201 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans [post]
func (h *LoanHandler) Create(c *fiber.Ctx) error {
	req, err := bindCreateLoanRequest(c)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	role, _ := c.Locals("role").(string)

	folder := fmt.Sprintf("LOANS/%s", uid.String())
	uploads := []struct {
		field string
		dest  **string
	}{
		{"selfie_document", &req.SelfieDocument},
		{"selfie_ktp_document", &req.SelfieKtpDocument},
		{"pks_document", &req.PksDocument},
		{"collateral_document", &req.CollateralDocument},
	}
	for _, u := range uploads {
		key, uerr := utils.UploadFormFile(c.Context(), c, u.field, folder)
		if uerr != nil {
			return utils.UploadErrorResponse(c, u.field, uerr)
		}
		if key != nil {
			*u.dest = key
		}
	}

	if req.SubmitImmediately {
		missing := []string{}
		if req.SelfieDocument == nil || *req.SelfieDocument == "" {
			missing = append(missing, "Selfie")
		}
		if req.SelfieKtpDocument == nil || *req.SelfieKtpDocument == "" {
			missing = append(missing, "Selfie + KTP")
		}
		if len(missing) > 0 {
			return utils.ErrorResponse(
				c,
				http.StatusBadRequest,
				"Dokumen wajib belum lengkap: "+strings.Join(missing, ", "),
			)
		}
	}

	r, err := h.service.Create(c.Context(), req, uid, role)
	if err != nil {
		return utils.ErrorResponse(c, statusForServiceError(err), err.Error())
	}
	return utils.SuccessResponse(c, http.StatusCreated, "loan created", r)
}

// bindCreateLoanRequest accepts both JSON and multipart payloads.
func bindCreateLoanRequest(c *fiber.Ctx) (*model.CreateLoanRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.CreateLoanRequest{
			Purpose:           utils.FormString(c, "purpose"),
			LoanAmount:        utils.FormFloat(c, "loan_amount", 0),
			TenorMonths:       utils.FormInt(c, "tenor_months", 0),
			SubmitImmediately: utils.FormBool(c, "submit_immediately", false),
		}
		if pid := utils.FormUUID(c, "personnel_id"); pid != uuid.Nil {
			req.PersonnelID = &pid
		}
		if lpid := utils.FormUUID(c, "loan_product_id"); lpid != uuid.Nil {
			req.LoanProductID = &lpid
		}
		if v := c.FormValue("deduct_from_payroll"); v != "" {
			b := utils.FormBool(c, "deduct_from_payroll", true)
			req.DeductFromPayroll = &b
		}
		// Default false (not omitted-means-accepted) — a missing field must
		// not be silently treated as consent given.
		termsAccepted := utils.FormBool(c, "terms_accepted", false)
		req.TermsAccepted = &termsAccepted
		if v := utils.FormString(c, "placement_bujp_name"); v != "" {
			req.PlacementBujpName = &v
		}
		if v := utils.FormString(c, "placement_address"); v != "" {
			req.PlacementAddress = &v
		}
		if v := utils.FormString(c, "placement_duration"); v != "" {
			req.PlacementDuration = &v
		}
		return req, nil
	}

	var req model.CreateLoanRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

// Update godoc
// @Summary      Update a draft loan
// @Tags         Loans
// @Accept       json
// @Produce      json
// @Param        id   path string true "ID"
// @Param        body body model.UpdateLoanRequest true "Body"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id} [put]
func (h *LoanHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	var req model.UpdateLoanRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	r, err := h.service.Update(c.Context(), id, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan updated", r)
}

// Submit godoc
// @Summary      Submit a draft loan for approval
// @Tags         Loans
// @Param        id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id}/submit [post]
func (h *LoanHandler) Submit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	r, err := h.service.Submit(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan submitted", r)
}

// Cancel godoc
// @Summary      Cancel a loan
// @Tags         Loans
// @Param        id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id}/cancel [post]
func (h *LoanHandler) Cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	body := struct {
		Reason *string `json:"reason"`
	}{}
	_ = c.BodyParser(&body)
	r, err := h.service.Cancel(c.Context(), id, body.Reason)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan cancelled", r)
}

// UserConfirm godoc
// @Summary      Borrower confirmation (accept/decline)
// @Tags         Loans
// @Accept       json
// @Param        id   path string true "Loan UUID"
// @Param        body body model.LoanUserConfirmationRequest true "Body"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id}/user-confirmation [post]
func (h *LoanHandler) UserConfirm(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	var req model.LoanUserConfirmationRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	r, err := h.service.UserConfirm(c.Context(), id, req.Action, req.Reason)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan confirmation processed", r)
}

// Statistics godoc
// @Summary      Loan statistics by status
// @Description  Returns counts grouped by loan status. Numbers are scoped by
//
//	role: company_admin/supervisor see only their BUJP, guard
//	sees only their own personnel, super_admin sees the whole
//	tenant.
//
// @Tags         Loans
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/statistics [get]
func (h *LoanHandler) Statistics(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	stats, err := h.service.StatisticsScoped(c.Context(), role, uid)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan statistics", stats)
}

// ExportBPRKS returns lifecycle loans in an Excel workbook for BPRKS.
// @Summary      Export BPRKS credit data
// @Description  Exports disbursed, active, and completed loans to an Excel workbook.
// @Tags         Loans
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Success      200 {file} file
// @Security     BearerAuth
// @Router       /api/v1/loans/export [get]
func (h *LoanHandler) ExportBPRKS(c *fiber.Ctx) error {
	if utils.CallerRole(c) != "bprks" {
		return forbiddenResponse(c)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	const pageSize = 1000
	filters := map[string]interface{}{
		"status_in": []string{
			model.LoanStatusDisbursed,
			model.LoanStatusActive,
			model.LoanStatusCompleted,
		},
	}
	loans := make([]model.LoanResponse, 0)
	for page := 1; ; page++ {
		batch, total, err := h.service.GetAll(ctx, page, pageSize, filters)
		if err != nil {
			return utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memuat data kredit")
		}
		loans = append(loans, batch...)
		if len(batch) < pageSize || int64(len(loans)) >= total {
			break
		}
	}

	data, err := bprksLoanWorkbook(loans)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat file Excel")
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="data-kredit-bprks.xlsx"`)
	return c.Send(data)
}

func bprksLoanWorkbook(loans []model.LoanResponse) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Kredit BPRKS"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []string{
		"No", "NIP", "NIK", "Nama Lengkap", "Rekening BJB", "Gaji Kotor",
		"No. Kredit", "Status", "Jumlah Pinjaman", "Bunga (%)", "Tenor (Bulan)",
		"Angsuran/Bulan", "Pembayaran",
	}
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	lastColumn, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return nil, err
	}

	safe := func(value *string) string {
		if value == nil {
			return ""
		}
		return spreadsheetSafe(*value)
	}
	for i, loan := range loans {
		var nip, nik, name, account, bank string
		var salary float64
		if loan.Personnel != nil {
			nip = safe(loan.Personnel.NIP)
			nik = spreadsheetSafe(loan.Personnel.IDNumber)
			name = spreadsheetSafe(loan.Personnel.FullName)
			account = safe(loan.Personnel.AccountNumber)
			bank = safe(loan.Personnel.BankName)
			salary = loan.Personnel.BaseSalary
		}
		if !strings.Contains(strings.ToLower(bank), "bjb") {
			account = "-"
		}
		amount := loan.LoanAmount
		if loan.ApprovedAmount != nil && *loan.ApprovedAmount > 0 {
			amount = *loan.ApprovedAmount
		}
		values := []interface{}{
			i + 1, nip, nik, name, account, salary, spreadsheetSafe(loan.LoanNumber),
			loan.Status, amount, loan.InterestRate, loan.TenorMonths,
			loan.MonthlyInstallment, loan.TotalRepayment,
		}
		for j, value := range values {
			cell, err := excelize.CoordinatesToCellName(j+1, i+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Delete godoc
// @Summary      Delete a draft loan
// @Tags         Loans
// @Param        id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{id} [delete]
func (h *LoanHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	if err := h.service.Delete(c.Context(), id); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan deleted", nil)
}

// canAccessLoan loads the loan and applies the same per-role tenant rules
// used by GetByID. Returns false when the caller has no access (or when the
// loan does not exist, in which case the caller will see a generic 403 to
// avoid leaking existence).
func (h *LoanHandler) canAccessLoan(c *fiber.Ctx, id uuid.UUID) bool {
	r, err := h.service.GetByID(c.Context(), id)
	if err != nil || r == nil {
		return false
	}
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	switch role {
	case "super_admin", "admin":
		return true
	case "company_admin", "supervisor":
		bid := h.service.ResolveUserBujpID(c.Context(), uid)
		return bid != uuid.Nil && r.BujpID != nil && *r.BujpID == bid
	case "guard":
		pid := h.service.ResolveUserPersonnelID(c.Context(), uid)
		return pid != uuid.Nil && r.PersonnelID == pid
	}
	return false
}
