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
)

type PayrollHandler struct {
	service services.PayrollService
}

func NewPayrollHandler(service services.PayrollService) *PayrollHandler {
	return &PayrollHandler{service: service}
}

// GetAll godoc
// @Summary Get all payrolls
// @Description Get list of payrolls with pagination and filters
// @Tags payrolls
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by Personnel ID"
// @Param bujp_id query string false "Filter by BUJP ID"
// @Param period query string false "Filter by period (YYYY-MM)"
// @Param payment_status query string false "Filter by payment status"
// @Success 200 {object} model.APIResponse{data=[]model.PayrollResponse}
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/payrolls [get]
// @Security BearerAuth
func (h *PayrollHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)

	filters := make(map[string]interface{})
	if personnelID := c.Query("personnel_id"); personnelID != "" {
		filters["personnel_id"] = personnelID
	}
	if bujpID := c.Query("bujp_id"); bujpID != "" {
		filters["bujp_id"] = bujpID
	}
	if period := c.Query("period"); period != "" {
		filters["period"] = period
	}
	if status := c.Query("payment_status"); status != "" {
		filters["payment_status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	payrolls, total, err := h.service.GetAll(ctx, page, limit, filters)
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
		Message: "Successfully retrieved payrolls",
		Data:    payrolls,
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
// @Summary Get payroll by ID
// @Description Get payroll details by ID with details
// @Tags payrolls
// @Produce json
// @Param id path string true "Payroll ID"
// @Success 200 {object} model.APIResponse{data=model.PayrollResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/payrolls/{id} [get]
// @Security BearerAuth
func (h *PayrollHandler) GetByID(c *fiber.Ctx) error {
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

	payroll, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
	}

	if !utils.CanAccessBujp(c, payroll.BujpID) {
		return forbiddenResponse(c)
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved payroll",
		Data:    payroll,
	})
}

// Create godoc
// @Summary Create new payroll
// @Description Create a new payroll with details
// @Tags payrolls
// @Accept json
// @Produce json
// @Param request body model.CreatePayrollRequest true "Payroll data"
// @Success 201 {object} model.APIResponse{data=model.PayrollResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/payrolls [post]
// @Security BearerAuth
func (h *PayrollHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req model.CreatePayrollRequest
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

	payroll, err := h.service.Create(ctx, &req, createdBy)
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
		Message: "Successfully created payroll",
		Data:    payroll,
	})
}

// Update godoc
// @Summary Update payroll
// @Description Update payroll by ID
// @Tags payrolls
// @Accept json
// @Produce json
// @Param id path string true "Payroll ID"
// @Param request body model.UpdatePayrollRequest true "Payroll data"
// @Success 200 {object} model.APIResponse{data=model.PayrollResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/payrolls/{id} [put]
// @Security BearerAuth
func (h *PayrollHandler) Update(c *fiber.Ctx) error {
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

	var req model.UpdatePayrollRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Tenant ownership check.
	existing, gerr := h.service.GetByID(ctx, id)
	if gerr != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Payroll not found",
		})
	}
	if !utils.CanAccessBujp(c, existing.BujpID) {
		return forbiddenResponse(c)
	}

	payroll, err := h.service.Update(ctx, id, &req)
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
		Message: "Successfully updated payroll",
		Data:    payroll,
	})
}

// Delete godoc
// @Summary Delete payroll
// @Description Delete payroll by ID
// @Tags payrolls
// @Produce json
// @Param id path string true "Payroll ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/payrolls/{id} [delete]
// @Security BearerAuth
func (h *PayrollHandler) Delete(c *fiber.Ctx) error {
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

	// Tenant ownership check before delete.
	existing, gerr := h.service.GetByID(ctx, id)
	if gerr != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Payroll not found",
		})
	}
	if !utils.CanAccessBujp(c, existing.BujpID) {
		return forbiddenResponse(c)
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
		Message: "Successfully deleted payroll",
	})
}

// Generate godoc
// @Summary Generate payrolls for a BUJP and period
// @Description Pro-rates base salary by attendance and applies active salary components
// @Tags payrolls
// @Accept json
// @Produce json
// @Param request body model.GeneratePayrollRequest true "Generate parameters"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/payrolls/generate [post]
// @Security BearerAuth
func (h *PayrollHandler) Generate(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var req model.GeneratePayrollRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Clamp BUJP for non-Pusat callers so they cannot generate for another tenant.
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		callerBujpID, _ := c.Locals("bujpID").(uuid.UUID)
		if callerBujpID == uuid.Nil {
			return c.Status(http.StatusForbidden).JSON(model.APIResponse{
				Status:  "error",
				Code:    http.StatusForbidden,
				Message: "Akun Anda tidak terhubung dengan BUJP manapun",
			})
		}
		req.BujpID = callerBujpID
	}

	var createdBy *uuid.UUID
	if userID := c.Locals("userID"); userID != nil {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	result, err := h.service.Generate(ctx, &req, createdBy)
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "wajib") || strings.Contains(msg, "format") {
			status = http.StatusBadRequest
		} else if strings.Contains(msg, "tidak ada personel") {
			status = http.StatusNotFound
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
		Message: "Payroll generated",
		Data:    result,
	})
}
