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

type LeaveHandler struct {
	service services.LeaveService
}

func NewLeaveHandler(service services.LeaveService) *LeaveHandler {
	return &LeaveHandler{
		service: service,
	}
}

// GetAll godoc
// @Summary Get all leaves
// @Tags Leaves
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by personnel ID"
// @Param location_id query string false "Filter by location ID"
// @Param status query string false "Filter by status"
// @Success 200 {object} model.APIResponse{data=[]model.Leave,meta=model.PaginationMeta}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /leaves [get]
// @Security BearerAuth
func (h *LeaveHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, limit := utils.ParsePagination(c)
	personnelID := c.Query("personnel_id")
	leaveType := c.Query("leave_type")
	status := c.Query("status")

	filters := make(map[string]interface{})
	if personnelID != "" {
		if pid, err := uuid.Parse(personnelID); err == nil {
			filters["personnel_id"] = pid
		}
	}
	if leaveType != "" {
		filters["type"] = leaveType
	}
	if status != "" {
		filters["status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	leaves, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to retrieve leaves",
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	hasNext := int64(page) < totalPages
	hasPrevious := page > 1

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Leaves retrieved successfully",
		Data:    leaves,
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
// @Summary Get leave by ID
// @Tags Leaves
// @Produce json
// @Param id path string true "Leave ID"
// @Success 200 {object} model.APIResponse{data=model.Leave}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /leaves/{id} [get]
// @Security BearerAuth
func (h *LeaveHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid leave ID",
		})
	}

	leave, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Leave not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Leave retrieved successfully",
		Data:    leave,
	})
}

// Create godoc
// @Summary Create new leave
// @Tags Leaves
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param leave body model.CreateLeaveRequest true "Leave data"
// @Success 201 {object} model.APIResponse{data=model.Leave}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /leaves [post]
// @Security BearerAuth
func (h *LeaveHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := bindCreateLeaveRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("LEAVES/%s", uid.String())
	for _, field := range []string{"attachment", "supporting_document"} {
		if key, uerr := utils.UploadFormFile(ctx, c, field, folder); uerr != nil {
			return utils.UploadErrorResponse(c, field, uerr)
		} else if key != nil {
			req.SupportingDocument = key
			break
		}
	}

	leave, err := h.service.Create(ctx, req)
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
		Code:    http.StatusCreated,
		Message: "Leave created successfully",
		Data:    leave,
	})
}

func bindCreateLeaveRequest(c *fiber.Ctx) (*model.CreateLeaveRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.CreateLeaveRequest{
			PersonnelID: utils.FormUUID(c, "personnel_id"),
			Type:        utils.FormString(c, "type"),
			StartDate:   utils.FormString(c, "start_date"),
			EndDate:     utils.FormString(c, "end_date"),
			Reason:      utils.FormString(c, "reason"),
		}
		if v := c.FormValue("total_days"); v != "" {
			n := utils.FormInt(c, "total_days", 0)
			req.TotalDays = &n
		}
		if v := utils.FormString(c, "supporting_document"); v != "" {
			req.SupportingDocument = &v
		}
		return req, nil
	}

	var req model.CreateLeaveRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

// Update godoc
// @Summary Update leave
// @Tags Leaves
// @Produce json
// @Param id path string true "Leave ID"
// @Param leave body model.UpdateLeaveRequest true "Leave data"
// @Success 200 {object} model.APIResponse{data=model.Leave}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /leaves/{id} [put]
// @Security BearerAuth
func (h *LeaveHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid leave ID",
		})
	}

	var req model.UpdateLeaveRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	leave, err := h.service.Update(ctx, id, &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to update leave",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Leave updated successfully",
		Data:    leave,
	})
}

// Delete godoc
// @Summary Delete leave
// @Tags Leaves
// @Produce json
// @Param id path string true "Leave ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /leaves/{id} [delete]
// @Security BearerAuth
func (h *LeaveHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid leave ID",
		})
	}

	if err := h.service.Delete(ctx, id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete leave",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Leave deleted successfully",
	})
}

// Pending godoc
// @Summary Get pending leaves awaiting approval
// @Tags Leaves
// @Produce json
// @Success 200 {object} model.APIResponse{data=[]model.Leave,meta=model.PaginationMeta}
// @Router /leaves/pending [get]
// @Security BearerAuth
func (h *LeaveHandler) Pending(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, limit := utils.ParsePagination(c)

	filters := make(map[string]interface{})
	utils.ApplyBujpScope(c, filters)

	leaves, total, err := h.service.GetPending(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError, Message: "Failed to retrieve pending leaves",
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status: "success", Code: http.StatusOK, Message: "Pending leaves retrieved", Data: leaves,
		Meta: &model.PaginationMeta{
			Page: page, Limit: limit, Total: total, TotalPages: int(totalPages),
			HasNext: int64(page) < totalPages, HasPrevious: page > 1,
		},
	})
}

// Approve godoc
// @Summary Approve or reject a leave request
// @Tags Leaves
// @Accept json
// @Produce json
// @Param id path string true "Leave ID"
// @Param body body model.ApproveLeaveRequest true "Approval payload"
// @Success 200 {object} model.APIResponse{data=model.Leave}
// @Router /leaves/{id}/approve [post]
// @Security BearerAuth
func (h *LeaveHandler) Approve(c *fiber.Ctx) error {
	return h.processApproval(c, "approved")
}

// Reject godoc
// @Summary Reject a leave request
// @Tags Leaves
// @Accept json
// @Produce json
// @Param id path string true "Leave ID"
// @Param body body model.ApproveLeaveRequest true "Rejection payload"
// @Success 200 {object} model.APIResponse{data=model.Leave}
// @Router /leaves/{id}/reject [post]
// @Security BearerAuth
func (h *LeaveHandler) Reject(c *fiber.Ctx) error {
	return h.processApproval(c, "rejected")
}

func (h *LeaveHandler) processApproval(c *fiber.Ctx, defaultStatus string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest, Message: "Invalid leave ID",
		})
	}

	var req model.ApproveLeaveRequest
	_ = c.BodyParser(&req)
	if req.Status == "" {
		req.Status = defaultStatus
	}
	if req.Status != "approved" && req.Status != "rejected" {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest, Message: "status must be 'approved' or 'rejected'",
		})
	}

	approverID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || approverID == uuid.Nil {
		return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
			Status: "error", Code: http.StatusUnauthorized, Message: "Unauthorized",
		})
	}

	notes := ""
	if req.ApproverNotes != nil {
		notes = *req.ApproverNotes
	}

	leave, err := h.service.Approve(ctx, id, approverID, req.Status, notes)
	if err != nil {
		status := statusForServiceError(err)
		return c.Status(status).JSON(model.APIResponse{
			Status: "error", Code: status, Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Leave " + req.Status + " successfully",
		Data:    leave,
	})
}
