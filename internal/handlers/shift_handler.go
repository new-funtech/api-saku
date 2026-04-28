package handlers

import (
	"fmt"
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ShiftHandler struct {
	service services.ShiftService
}

func NewShiftHandler(service services.ShiftService) *ShiftHandler {
	return &ShiftHandler{service: service}
}

// GetAllShifts godoc
// @Summary      Get all shifts
// @Description  Retrieve a paginated list of all shifts
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        page    query     int     false  "Page number" default(1)
// @Param        limit   query     int     false  "Items per page" default(10)
// @Param        bujp_id query     string  false  "Filter by BUJP ID"
// @Param        status  query     string  false  "Filter by status (active/inactive)"
// @Param        search  query     string  false  "Search by name"
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/shifts [get]
func (h *ShiftHandler) GetAllShifts(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)

	// Enforce maximum limit to prevent database overload
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	filters := make(map[string]interface{})
	if bujpIDStr := c.Query("bujp_id"); bujpIDStr != "" {
		if bujpID, err := uuid.Parse(bujpIDStr); err == nil {
			filters["bujp_id"] = bujpID
		}
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}

	utils.ApplyBujpScope(c, filters)

	shifts, total, err := h.service.GetAll(c.Context(), page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrInternalServer,
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)

	meta := &model.PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  int(totalPages),
		HasNext:     page < int(totalPages),
		HasPrevious: page > 1,
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved shifts",
		Data:    shifts,
		Meta:    meta,
	})
}

// GetShiftByID godoc
// @Summary      Get shift by ID
// @Description  Retrieve a single shift by its UUID
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Shift UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/shifts/{id} [get]
func (h *ShiftHandler) GetShiftByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	shift, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Shift not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved shift",
		Data:    shift,
	})
}

// CreateShift godoc
// @Summary      Create a new shift
// @Description  Create a new shift with the provided data
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateShiftRequest  true  "Shift data"
// @Success      201  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/shifts [post]
func (h *ShiftHandler) CreateShift(c *fiber.Ctx) error {
	var req model.CreateShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	shift, err := h.service.Create(c.Context(), &req)
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
		Message: "Shift created successfully",
		Data:    shift,
	})
}

// UpdateShift godoc
// @Summary      Update a shift
// @Description  Update an existing shift by ID
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        id       path      string                   true  "Shift UUID"
// @Param        request  body      model.UpdateShiftRequest true  "Updated shift data"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/shifts/{id} [put]
func (h *ShiftHandler) UpdateShift(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	var req model.UpdateShiftRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	shift, err := h.service.Update(c.Context(), id, &req)
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
		Message: "Shift updated successfully",
		Data:    shift,
	})
}

// DeleteShift godoc
// @Summary      Delete a shift
// @Description  Delete a shift by ID
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Shift UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/shifts/{id} [delete]
func (h *ShiftHandler) DeleteShift(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	if err := h.service.Delete(c.Context(), id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Shift deleted successfully",
	})
}
