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

type BujpHandler struct {
	service services.BujpService
}

func NewBujpHandler(service services.BujpService) *BujpHandler {
	return &BujpHandler{service: service}
}

// GetAllBujps godoc
// @Summary      Get all BUJPs
// @Description  Retrieve a paginated list of all BUJPs
// @Tags         Bujps
// @Accept       json
// @Produce      json
// @Param        page   query     int     false  "Page number" default(1)
// @Param        limit  query     int     false  "Items per page" default(10)
// @Param        status query     string  false  "Filter by status (active/inactive)"
// @Param        search query     string  false  "Search by code or name"
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/bujps [get]
func (h *BujpHandler) GetAllBujps(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)

	filters := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}

	bujps, total, err := h.service.GetAll(c.Context(), page, limit, filters)
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
		Message: "Successfully retrieved BUJPs",
		Data:    bujps,
		Meta:    meta,
	})
}

// GetBujpByID godoc
// @Summary      Get BUJP by ID
// @Description  Retrieve a single BUJP by its UUID
// @Tags         Bujps
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "BUJP UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/bujps/{id} [get]
func (h *BujpHandler) GetBujpByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	bujp, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "BUJP not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved BUJP",
		Data:    bujp,
	})
}

// CreateBujp godoc
// @Summary      Create a new BUJP
// @Description  Create a new BUJP with the provided data
// @Tags         Bujps
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateBujpRequest  true  "BUJP data"
// @Success      201  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/bujps [post]
func (h *BujpHandler) CreateBujp(c *fiber.Ctx) error {
	var req model.CreateBujpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	// Validate required fields
	if req.Code == "" {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Field 'code' is required",
			Data:    nil,
		})
	}
	if req.Name == "" {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Field 'name' is required",
			Data:    nil,
		})
	}

	bujp, err := h.service.Create(c.Context(), &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
			Data:    nil,
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: "BUJP created successfully",
		Data:    bujp,
	})
}

// UpdateBujp godoc
// @Summary      Update a BUJP
// @Description  Update an existing BUJP by ID
// @Tags         Bujps
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "BUJP UUID"
// @Param        request  body      model.UpdateBujpRequest true  "Updated BUJP data"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/bujps/{id} [put]
func (h *BujpHandler) UpdateBujp(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	var req model.UpdateBujpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	bujp, err := h.service.Update(c.Context(), id, &req)
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
		Message: "BUJP updated successfully",
		Data:    bujp,
	})
}

// DeleteBujp godoc
// @Summary      Delete a BUJP
// @Description  Delete a BUJP by ID
// @Tags         Bujps
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "BUJP UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/bujps/{id} [delete]
func (h *BujpHandler) DeleteBujp(c *fiber.Ctx) error {
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
		Message: "BUJP deleted successfully",
	})
}
