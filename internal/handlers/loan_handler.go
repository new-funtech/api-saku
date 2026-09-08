package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

	// Upload optional document files when sent as multipart parts.
	folder := fmt.Sprintf("LOANS/%s", uid.String())
	uploads := []struct {
		field string
		dest  **string
	}{
		{"ktp_document", &req.KtpDocument},
		{"npwp_document", &req.NpwpDocument},
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
		if req.KtpDocument == nil || *req.KtpDocument == "" {
			missing = append(missing, "KTP")
		}
		if req.NpwpDocument == nil || *req.NpwpDocument == "" {
			missing = append(missing, "NPWP")
		}
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
		if v := c.FormValue("interest_rate"); v != "" {
			r := utils.FormFloat(c, "interest_rate", 0)
			req.InterestRate = &r
		}
		if v := c.FormValue("deduct_from_payroll"); v != "" {
			b := utils.FormBool(c, "deduct_from_payroll", true)
			req.DeductFromPayroll = &b
		}
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
