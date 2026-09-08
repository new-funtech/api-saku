package handlers

import (
	"context"
	"encoding/json"
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

type PatrolHandler struct {
	service services.PatrolService
}

func NewPatrolHandler(service services.PatrolService) *PatrolHandler {
	return &PatrolHandler{
		service: service,
	}
}

// GetAll godoc
// @Summary Get all patrols
// @Tags Patrols
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by personnel ID"
// @Param location_id query string false "Filter by location ID"
// @Param status query string false "Filter by status"
// @Success 200 {object} model.APIResponse{data=[]model.Patrol,meta=model.PaginationMeta}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /patrols [get]
// @Security BearerAuth
func (h *PatrolHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)
	personnelID := c.Query("personnel_id")
	locationID := c.Query("location_id")
	status := c.Query("status")

	filters := make(map[string]interface{})
	if personnelID != "" {
		if pid, err := uuid.Parse(personnelID); err == nil {
			filters["personnel_id"] = pid
		}
	}
	if locationID != "" {
		if lid, err := uuid.Parse(locationID); err == nil {
			filters["location_id"] = lid
		}
	}
	if status != "" {
		filters["status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	patrols, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to retrieve patrols",
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	hasNext := int64(page) < totalPages
	hasPrevious := page > 1

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrols retrieved successfully",
		Data:    patrols,
		Meta: &model.PaginationMeta{
			Page:        page,
			Limit:       limit,
			Total:       total,
			TotalPages:  int(totalPages),
			HasNext:     hasNext,
			HasPrevious: hasPrevious,
		},
	})
}

// GetByID godoc
// @Summary Get patrol by ID
// @Tags Patrols
// @Produce json
// @Param id path string true "Patrol ID"
// @Success 200 {object} model.APIResponse{data=model.Patrol}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /patrols/{id} [get]
// @Security BearerAuth
func (h *PatrolHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid patrol ID",
		})
	}

	patrol, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Patrol not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrol retrieved successfully",
		Data:    patrol,
	})
}

// Create godoc
// @Summary Create new patrol
// @Tags Patrols
// @Produce json
// @Param patrol body model.CreatePatrolRequest true "Patrol data"
// @Success 201 {object} model.APIResponse{data=model.Patrol}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /patrols [post]
// @Security BearerAuth
func (h *PatrolHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := bindCreatePatrolRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("PATROLS/%s", uid.String())
	if key, uerr := utils.UploadFormFile(ctx, c, "photo", folder); uerr != nil {
		return utils.UploadErrorResponse(c, "photo", uerr)
	} else if key != nil {
		req.Photo = key
	}

	patrol, err := h.service.Create(ctx, req)
	if err != nil {
		status := statusForServiceError(err)
		return c.Status(status).JSON(model.APIResponse{
			Status:  "error",
			Code:    status,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrol created successfully",
		Data:    patrol,
	})
}

func bindCreatePatrolRequest(c *fiber.Ctx) (*model.CreatePatrolRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.CreatePatrolRequest{
			PersonnelID: utils.FormUUID(c, "personnel_id"),
			LocationID:  utils.FormUUID(c, "location_id"),
			Date:        utils.FormString(c, "date"),
			PatrolTime:  utils.FormString(c, "patrol_time"),
		}
		if aid := utils.FormUUID(c, "attendance_id"); aid != uuid.Nil {
			req.AttendanceID = &aid
		}
		if v := utils.FormString(c, "patrol_area"); v != "" {
			req.PatrolArea = &v
		}
		if v := strings.TrimSpace(c.FormValue("latitude")); v != "" {
			f := utils.FormFloat(c, "latitude", 0)
			req.Latitude = &f
		}
		if v := strings.TrimSpace(c.FormValue("longitude")); v != "" {
			f := utils.FormFloat(c, "longitude", 0)
			req.Longitude = &f
		}
		if v := utils.FormString(c, "notes"); v != "" {
			req.Notes = &v
		}
		if v := utils.FormString(c, "photo"); v != "" {
			req.Photo = &v
		}
		return req, nil
	}

	var req model.CreatePatrolRequest
	body := coerceFloatFields(c.Body(), "latitude", "longitude")
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// Update godoc
// @Summary Update patrol
// @Tags Patrols
// @Produce json
// @Param id path string true "Patrol ID"
// @Param patrol body model.UpdatePatrolRequest true "Patrol data"
// @Success 200 {object} model.APIResponse{data=model.Patrol}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /patrols/{id} [put]
// @Security BearerAuth
func (h *PatrolHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid patrol ID",
		})
	}

	var req model.UpdatePatrolRequest
	body := coerceFloatFields(c.Body(), "latitude", "longitude")
	if err := json.Unmarshal(body, &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	patrol, err := h.service.Update(ctx, id, &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to update patrol",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrol updated successfully",
		Data:    patrol,
	})
}

// Delete godoc
// @Summary Delete patrol
// @Tags Patrols
// @Produce json
// @Param id path string true "Patrol ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /patrols/{id} [delete]
// @Security BearerAuth
func (h *PatrolHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid patrol ID",
		})
	}

	if err := h.service.Delete(ctx, id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete patrol",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrol deleted successfully",
	})
}

// Validate godoc
// @Summary      Approve or reject a pending patrol record
// @Tags         Patrols
// @Accept       json
// @Produce      json
// @Param        id   path string                       true "Patrol ID"
// @Param        body body model.ValidatePatrolRequest  true "Validation payload"
// @Success      200 {object} model.APIResponse{data=model.PatrolResponse}
// @Failure      400 {object} model.APIResponse
// @Failure      404 {object} model.APIResponse
// @Failure      409 {object} model.APIResponse
// @Router       /patrols/{id}/validate [post]
// @Security     BearerAuth
func (h *PatrolHandler) Validate(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest, Message: "Invalid patrol ID",
		})
	}

	var req model.ValidatePatrolRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest, Message: "Invalid request body",
		})
	}
	switch strings.ToLower(strings.TrimSpace(req.ValidationStatus)) {
	case "validated", "approve":
		req.ValidationStatus = "approved"
	case "reject":
		req.ValidationStatus = "rejected"
	}
	if req.ValidationStatus != "approved" && req.ValidationStatus != "rejected" {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest,
			Message: "validation_status harus 'approved' atau 'rejected'",
		})
	}

	validatorID, _ := c.Locals("userID").(uuid.UUID)
	if validatorID == uuid.Nil {
		return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
			Status: "error", Code: http.StatusUnauthorized, Message: "Unauthorized",
		})
	}

	updated, err := h.service.Validate(ctx, id, validatorID, req.ValidationStatus)
	if err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status: "error", Code: statusCode, Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Patrol validation updated successfully",
		Data:    updated,
	})
}
