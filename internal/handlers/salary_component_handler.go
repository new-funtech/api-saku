package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type SalaryComponentHandler struct {
	service services.SalaryComponentService
}

func NewSalaryComponentHandler(service services.SalaryComponentService) *SalaryComponentHandler {
	return &SalaryComponentHandler{service: service}
}

// GetAll godoc
// @Summary Get all salary components
// @Description Get list of salary components with pagination and filters
// @Tags salary-components
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param bujp_id query string false "Filter by BUJP ID"
// @Param type query string false "Filter by type (allowance/deduction)"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} model.APIResponse{data=[]model.SalaryComponentResponse}
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/salary-components [get]
// @Security BearerAuth
func (h *SalaryComponentHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)

	filters := make(map[string]interface{})
	if bujpID := c.Query("bujp_id"); bujpID != "" {
		filters["bujp_id"] = bujpID
	}
	if componentType := c.Query("type"); componentType != "" {
		filters["type"] = componentType
	}
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		filters["is_active"] = isActiveStr == "true"
	}

	utils.ApplyBujpScope(c, filters)

	components, total, err := h.service.GetAll(ctx, page, limit, filters)
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
		Message: "Successfully retrieved salary components",
		Data:    components,
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
// @Summary Get salary component by ID
// @Description Get salary component details by ID
// @Tags salary-components
// @Produce json
// @Param id path string true "Salary Component ID"
// @Success 200 {object} model.APIResponse{data=model.SalaryComponentResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/salary-components/{id} [get]
// @Security BearerAuth
func (h *SalaryComponentHandler) GetByID(c *fiber.Ctx) error {
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

	component, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
	}

	if !utils.CanAccessOptionalBujp(c, component.BujpID) {
		return forbiddenResponse(c)
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved salary component",
		Data:    component,
	})
}

// Create godoc
// @Summary Create new salary component
// @Description Create a new salary component
// @Tags salary-components
// @Accept json
// @Produce json
// @Param request body model.CreateSalaryComponentRequest true "Salary Component data"
// @Success 201 {object} model.APIResponse{data=model.SalaryComponentResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/salary-components [post]
// @Security BearerAuth
func (h *SalaryComponentHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req model.CreateSalaryComponentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Tenant clamp: BUJP-scoped callers may only create components under
	// their own BUJP. Pusat may target any BUJP or create global (nil) ones.
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		if scoped, ok := c.Locals("bujpID").(uuid.UUID); ok && scoped != uuid.Nil {
			req.BujpID = &scoped
		}
	}

	component, err := h.service.Create(ctx, &req)
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
		Message: "Successfully created salary component",
		Data:    component,
	})
}

// Update godoc
// @Summary Update salary component
// @Description Update salary component by ID
// @Tags salary-components
// @Accept json
// @Produce json
// @Param id path string true "Salary Component ID"
// @Param request body model.UpdateSalaryComponentRequest true "Salary Component data"
// @Success 200 {object} model.APIResponse{data=model.SalaryComponentResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/salary-components/{id} [put]
// @Security BearerAuth
func (h *SalaryComponentHandler) Update(c *fiber.Ctx) error {
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

	var req model.UpdateSalaryComponentRequest
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
			Message: "Salary component not found",
		})
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
		return forbiddenResponse(c)
	}

	component, err := h.service.Update(ctx, id, &req)
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
		Message: "Successfully updated salary component",
		Data:    component,
	})
}

// Delete godoc
// @Summary Delete salary component
// @Description Delete salary component by ID
// @Tags salary-components
// @Produce json
// @Param id path string true "Salary Component ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/salary-components/{id} [delete]
// @Security BearerAuth
func (h *SalaryComponentHandler) Delete(c *fiber.Ctx) error {
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
			Message: "Salary component not found",
		})
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
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
		Message: "Successfully deleted salary component",
	})
}
