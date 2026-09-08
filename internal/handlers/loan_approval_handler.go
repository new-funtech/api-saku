package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type LoanApprovalHandler struct {
	service services.LoanApprovalService
}

func NewLoanApprovalHandler(service services.LoanApprovalService) *LoanApprovalHandler {
	return &LoanApprovalHandler{service: service}
}

// Pending godoc
// @Summary      List pending approvals for current role
// @Tags         LoanApprovals
// @Produce      json
// @Param        page  query int false "Page"
// @Param        limit query int false "Limit"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/pending [get]
func (h *LoanApprovalHandler) Pending(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)

	var bujpID *uuid.UUID
	switch role {
	case "company_admin", "supervisor":
		bid := h.service.ResolveUserBujpID(c.Context(), uid)
		if bid == uuid.Nil {
			return utils.SuccessResponseWithMeta(c, http.StatusOK, "pending approvals retrieved", []model.LoanApprovalResponse{}, &model.PaginationMeta{
				Page: page, Limit: limit, Total: 0, TotalPages: 0, HasNext: false, HasPrevious: false,
			})
		}
		bujpID = &bid
	default:
		if v := c.Query("bujp_id"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				bujpID = &id
			}
		}
	}

	items, total, err := h.service.GetPending(c.Context(), role, bujpID, page, limit)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "pending approvals retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

func (h *LoanApprovalHandler) canAccessLoan(c *fiber.Ctx, loanID uuid.UUID) bool {
	bujpID, personnelID, err := h.service.LoadLoanScope(c.Context(), loanID)
	if err != nil {
		return false
	}
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	switch role {
	case "super_admin", "admin", "bprks":
		return true
	case "company_admin", "supervisor":
		bid := h.service.ResolveUserBujpID(c.Context(), uid)
		return bid != uuid.Nil && bujpID != nil && *bujpID == bid
	case "guard":
		pid := h.service.ResolveUserPersonnelID(c.Context(), uid)
		return pid != uuid.Nil && personnelID == pid
	}
	return false
}

// ByLoan godoc
// @Summary      List approvals for a loan
// @Tags         LoanApprovals
// @Param        loan_id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/loan/{loan_id} [get]
func (h *LoanApprovalHandler) ByLoan(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("loan_id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	items, err := h.service.GetByLoan(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "approvals retrieved", items)
}

// Process godoc
// @Summary      Approve or reject a pending approval
// @Tags         LoanApprovals
// @Accept       json
// @Param        id   path string true "Approval UUID"
// @Param        body body model.LoanApprovalRequest true "Body"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/{id}/process [post]
func (h *LoanApprovalHandler) Process(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	var req model.LoanApprovalRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	approverID, _ := c.Locals("userID").(uuid.UUID)
	r, err := h.service.Process(c.Context(), id, approverID, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "approval processed", r)
}

// ApproveByLoan godoc
// @Summary      Approve the next pending approval for a loan (convenience for admin)
// @Tags         LoanApprovals
// @Accept       json
// @Param        loan_id path string true "Loan UUID"
// @Param        body body model.LoanApprovalRequest false "Body (notes, approved_amount, approved_tenor)"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/{loan_id}/approve [post]
func (h *LoanApprovalHandler) ApproveByLoan(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("loan_id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	var req model.LoanApprovalRequest
	_ = c.BodyParser(&req)
	req.Action = "approve"
	approverID, _ := c.Locals("userID").(uuid.UUID)
	role, _ := c.Locals("role").(string)
	if r, perr := h.service.Process(c.Context(), id, approverID, &req); perr == nil {
		return utils.SuccessResponse(c, http.StatusOK, "approval processed", r)
	} else if perr.Error() != "approval not found" {
		return utils.ErrorResponse(c, http.StatusBadRequest, perr.Error())
	}
	r, err := h.service.ProcessByLoan(c.Context(), id, approverID, role, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "approval processed", r)
}

// RejectByLoan godoc
// @Summary      Reject the next pending approval for a loan (convenience for admin)
// @Tags         LoanApprovals
// @Accept       json
// @Param        loan_id path string true "Loan UUID"
// @Param        body body model.LoanApprovalRequest false "Body (notes)"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/{loan_id}/reject [post]
func (h *LoanApprovalHandler) RejectByLoan(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("loan_id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	var req model.LoanApprovalRequest
	_ = c.BodyParser(&req)
	req.Action = "reject"
	approverID, _ := c.Locals("userID").(uuid.UUID)
	role, _ := c.Locals("role").(string)
	if r, perr := h.service.Process(c.Context(), id, approverID, &req); perr == nil {
		return utils.SuccessResponse(c, http.StatusOK, "approval processed", r)
	} else if perr.Error() != "approval not found" {
		return utils.ErrorResponse(c, http.StatusBadRequest, perr.Error())
	}
	r, err := h.service.ProcessByLoan(c.Context(), id, approverID, role, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "approval processed", r)
}

// Disburse godoc
// @Tags         LoanApprovals
// @Param        loan_id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-approvals/loan/{loan_id}/disburse [post]
func (h *LoanApprovalHandler) Disburse(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("loan_id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	if !h.canAccessLoan(c, id) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	r, err := h.service.Disburse(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan disbursed", r)
}
