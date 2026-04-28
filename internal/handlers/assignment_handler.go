package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AssignmentHandler struct {
	service services.AssignmentService
}

func NewAssignmentHandler(service services.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{
		service: service,
	}
}

// GetAll godoc
// @Summary Get all assignments
// @Tags Assignments
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by personnel ID"
// @Param location_id query string false "Filter by location ID"
// @Param status query string false "Filter by status"
// @Success 200 {object} model.APIResponse{data=[]model.Assignment,meta=model.PaginationMeta}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /assignments [get]
// @Security BearerAuth
func (h *AssignmentHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)
	personnelID := c.Query("personnel_id")
	locationID := c.Query("location_id")
	status := c.Query("status")

	filters := make(map[string]interface{})
	if personnelID != "" {
		if parsed, err := uuid.Parse(personnelID); err == nil {
			filters["personnel_id"] = parsed
		}
	}
	if locationID != "" {
		if parsed, err := uuid.Parse(locationID); err == nil {
			filters["location_id"] = parsed
		}
	}
	if status != "" {
		filters["status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	assignments, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		// Log the underlying error so 500s are debuggable; the response body
		// keeps a sanitized message for non-server errors and a generic one
		// for true server errors.
		statusCode := statusForServiceError(err)
		log.Printf("assignments.GetAll error: %v", err)
		msg := err.Error()
		if statusCode == http.StatusInternalServerError {
			msg = fmt.Sprintf("Failed to retrieve assignments: %s", err.Error())
		}
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: msg,
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	hasNext := int64(page) < totalPages
	hasPrevious := page > 1

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Assignments retrieved successfully",
		Data:    assignments,
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
// @Summary Get assignment by ID
// @Tags Assignments
// @Produce json
// @Param id path string true "Assignment ID"
// @Success 200 {object} model.APIResponse{data=model.Assignment}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /assignments/{id} [get]
// @Security BearerAuth
func (h *AssignmentHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid assignment ID",
		})
	}

	assignment, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Assignment not found",
		})
	}

	// Tenant ownership check via the assignment's personnel.
	if !utils.CanAccessPersonnel(c, assignment.PersonnelID, assignment.Personnel.BujpID) {
		return forbiddenResponse(c)
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Assignment retrieved successfully",
		Data:    assignment,
	})
}

// Create godoc
// @Summary Create new assignment
// @Tags Assignments
// @Produce json
// @Param assignment body model.CreateAssignmentRequest true "Assignment data"
// @Success 201 {object} model.APIResponse{data=model.Assignment}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /assignments [post]
// @Security BearerAuth
func (h *AssignmentHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req model.CreateAssignmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Stamp the authenticated user as the creator so the audit trail and the
	// assignment.created_at column are consistent (clients should not be
	// trusted to set CreatedBy themselves).
	if req.CreatedBy == nil {
		if uid, ok := c.Locals("userID").(uuid.UUID); ok && uid != uuid.Nil {
			req.CreatedBy = &uid
		}
	}

	assignment, err := h.service.Create(ctx, &req)
	if err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Assignment created successfully",
		Data:    assignment,
	})
}

// Update godoc
// @Summary Update assignment
// @Tags Assignments
// @Produce json
// @Param id path string true "Assignment ID"
// @Param assignment body model.UpdateAssignmentRequest true "Assignment data"
// @Success 200 {object} model.APIResponse{data=model.Assignment}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /assignments/{id} [put]
// @Security BearerAuth
func (h *AssignmentHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid assignment ID",
		})
	}

	var req model.UpdateAssignmentRequest
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
			Message: "Assignment not found",
		})
	}
	if !utils.CanAccessPersonnel(c, existing.PersonnelID, existing.Personnel.BujpID) {
		return forbiddenResponse(c)
	}

	assignment, err := h.service.Update(ctx, id, &req)
	if err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Assignment updated successfully",
		Data:    assignment,
	})
}

// Delete godoc
// @Summary Delete assignment
// @Tags Assignments
// @Produce json
// @Param id path string true "Assignment ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /assignments/{id} [delete]
// @Security BearerAuth
func (h *AssignmentHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid assignment ID",
		})
	}

	// Tenant ownership check before delete.
	existing, gerr := h.service.GetByID(ctx, id)
	if gerr != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Assignment not found",
		})
	}
	if !utils.CanAccessPersonnel(c, existing.PersonnelID, existing.Personnel.BujpID) {
		return forbiddenResponse(c)
	}

	if err := h.service.Delete(ctx, id); err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Assignment deleted successfully",
	})
}
