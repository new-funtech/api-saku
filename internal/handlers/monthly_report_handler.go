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
