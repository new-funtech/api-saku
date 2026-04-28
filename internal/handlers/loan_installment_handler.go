package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type LoanInstallmentHandler struct {
	service     services.LoanInstallmentService
	loanService services.LoanService
}

func NewLoanInstallmentHandler(service services.LoanInstallmentService, loanService services.LoanService) *LoanInstallmentHandler {
	return &LoanInstallmentHandler{service: service, loanService: loanService}
}

// ByLoan godoc
// @Summary      List installments for a loan with summary
// @Tags         LoanInstallments
// @Param        loan_id path string true "Loan UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loans/{loan_id}/installments [get]
func (h *LoanInstallmentHandler) ByLoan(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("loan_id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}

	// Enforce per-role access: pusat (always), bujp-scoped (match loan.BujpID),
	// guard (match loan.PersonnelID). Mirrors LoanHandler.canAccessLoan.
	loan, lerr := h.loanService.GetByID(c.Context(), id)
	if lerr != nil || loan == nil {
		return utils.ErrorResponse(c, http.StatusNotFound, "loan not found")
	}
	if !h.canAccessLoan(c, loan) {
		return utils.ErrorResponse(c, http.StatusForbidden, "you do not have access to this loan")
	}

	items, err := h.service.GetByLoan(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}

	var totalPaid, totalRemaining float64
	var paid, pending, overdue int
	for _, it := range items {
		totalPaid += it.PaidAmount
		totalRemaining += it.RemainingAmount
		switch it.Status {
		case model.LoanInstallmentStatusPaid:
			paid++
		case model.LoanInstallmentStatusOverdue:
			overdue++
		default:
			pending++
		}
	}
	summary := fiber.Map{
		"total_installments":   len(items),
		"paid_installments":    paid,
		"pending_installments": pending,
		"overdue_installments": overdue,
		"total_paid":           totalPaid,
		"total_remaining":      totalRemaining,
	}

	return utils.SuccessResponse(c, http.StatusOK, "installments retrieved", fiber.Map{
		"loan":         loan,
		"installments": items,
		"summary":      summary,
	})
}

// canAccessLoan applies per-role rules: pusat sees all; bujp-scoped roles
// match loan.BujpID; guards match loan.PersonnelID. Mirrors LoanHandler.canAccessLoan.
func (h *LoanInstallmentHandler) canAccessLoan(c *fiber.Ctx, loan *model.LoanResponse) bool {
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("userID").(uuid.UUID)
	switch role {
	case "super_admin", "admin":
		return true
	case "company_admin", "supervisor":
		bid := h.loanService.ResolveUserBujpID(c.Context(), uid)
		return bid != uuid.Nil && loan.BujpID != nil && *loan.BujpID == bid
	case "guard":
		pid := h.loanService.ResolveUserPersonnelID(c.Context(), uid)
		return pid != uuid.Nil && loan.PersonnelID == pid
	}
	return false
}

// Pay godoc
// @Summary      Record a payment for an installment
// @Tags         LoanInstallments
// @Accept       json
// @Param        id   path string true "Installment UUID"
// @Param        body body model.LoanInstallmentPaymentRequest true "Body"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-installments/{id}/pay [post]
func (h *LoanInstallmentHandler) Pay(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	var req model.LoanInstallmentPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	r, err := h.service.Pay(c.Context(), id, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "installment paid", r)
}

// MarkOverdue godoc
// @Summary      Mark all overdue installments (cron-like manual trigger)
// @Tags         LoanInstallments
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-installments/mark-overdue [post]
func (h *LoanInstallmentHandler) MarkOverdue(c *fiber.Ctx) error {
	count, err := h.service.MarkOverdue(c.Context())
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "overdue installments updated", model.APIResponse{Data: map[string]int64{"updated": count}})
}
