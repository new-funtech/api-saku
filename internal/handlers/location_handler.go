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

type LocationHandler struct {
	service services.LocationService
}

func NewLocationHandler(service services.LocationService) *LocationHandler {
	return &LocationHandler{service: service}
}

// GetAllLocations godoc
// @Summary      Get all locations
// @Description  Retrieve a paginated list of all locations
// @Tags         Locations
// @Accept       json
// @Produce      json
// @Param        page    query     int     false  "Page number" default(1)
// @Param        limit   query     int     false  "Items per page" default(10)
// @Param        bujp_id query     string  false  "Filter by BUJP ID"
// @Param        status  query     string  false  "Filter by status (active/inactive)"
// @Param        search  query     string  false  "Search by code, name, or address"
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/locations [get]
func (h *LocationHandler) GetAllLocations(c *fiber.Ctx) error {
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

	locations, total, err := h.service.GetAll(c.Context(), page, limit, filters)
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
		Message: "Successfully retrieved locations",
		Data:    locations,
		Meta:    meta,
	})
}

// GetLocationByID godoc
// @Summary      Get location by ID
// @Description  Retrieve a single location by its UUID
// @Tags         Locations
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Location UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/locations/{id} [get]
func (h *LocationHandler) GetLocationByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	location, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Location not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved location",
		Data:    location,
	})
}

// CreateLocation godoc
// @Summary      Create a new location
// @Description  Create a new location with the provided data
// @Tags         Locations
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateLocationRequest  true  "Location data"
// @Success      201  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/locations [post]
func (h *LocationHandler) CreateLocation(c *fiber.Ctx) error {
	var req model.CreateLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	location, err := h.service.Create(c.Context(), &req)
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
		Message: "Location created successfully",
		Data:    location,
	})
}

// UpdateLocation godoc
// @Summary      Update a location
// @Description  Update an existing location by ID
// @Tags         Locations
// @Accept       json
// @Produce      json
// @Param        id       path      string                      true  "Location UUID"
// @Param        request  body      model.UpdateLocationRequest true  "Updated location data"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/locations/{id} [put]
func (h *LocationHandler) UpdateLocation(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	var req model.UpdateLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	location, err := h.service.Update(c.Context(), id, &req)
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
		Message: "Location updated successfully",
		Data:    location,
	})
}

// DeleteLocation godoc
// @Summary      Delete a location
// @Description  Delete a location by ID
// @Tags         Locations
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Location UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/locations/{id} [delete]
func (h *LocationHandler) DeleteLocation(c *fiber.Ctx) error {
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
		Message: "Location deleted successfully",
	})
}
