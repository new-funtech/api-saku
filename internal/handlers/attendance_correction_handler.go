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

const defaultTimeout = 10 * time.Second

type AttendanceCorrectionHandler struct {
	service services.AttendanceCorrectionService
}

func NewAttendanceCorrectionHandler(service services.AttendanceCorrectionService) *AttendanceCorrectionHandler {
	return &AttendanceCorrectionHandler{
		service: service,
	}
}

// GetAll godoc
// @Summary Get all attendance corrections
// @Tags Attendance Corrections
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by personnel ID"
// @Param location_id query string false "Filter by location ID"
// @Param status query string false "Filter by status"
// @Success 200 {object} model.APIResponse{data=[]model.AttendanceCorrection,meta=model.PaginationMeta}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendance-corrections [get]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	// Validate and normalize pagination
	page, limit = utils.ValidatePagination(page, limit)

	filters := make(map[string]interface{})
	if personnelID := c.Query("personnel_id"); personnelID != "" {
		id, err := uuid.Parse(personnelID)
		if err != nil {
			return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid personnel ID format")
		}
		filters["personnel_id"] = id
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	corrections, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return utils.HandleError(c, err)
	}

	totalPages, hasNext, hasPrevious := utils.CalculatePaginationMeta(page, limit, total)

	return utils.SuccessResponseWithMeta(c, http.StatusOK, "Attendance corrections retrieved successfully", corrections, &model.PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     hasNext,
		HasPrevious: hasPrevious,
	})
}

// GetByID godoc
// @Summary Get attendance correction by ID
// @Tags Attendance Corrections
// @Produce json
// @Param id path string true "Attendance Correction ID"
// @Success 200 {object} model.APIResponse{data=model.AttendanceCorrection}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendance-corrections/{id} [get]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid attendance correction ID format")
	}

	correction, err := h.service.GetByID(ctx, id)
	if err != nil {
		return utils.HandleError(c, err)
	}

	return utils.SuccessResponse(c, http.StatusOK, "Attendance correction retrieved successfully", correction)
}

// Create godoc
// @Summary Create new attendance correction
// @Tags Attendance Corrections
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param correction body model.CreateAttendanceCorrectionRequest true "Attendance Correction data"
// @Success 201 {object} model.APIResponse{data=model.AttendanceCorrection}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendance-corrections [post]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	req, err := bindCreateAttendanceCorrectionRequest(c)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("ATTENDANCE_CORRECTIONS/%s", uid.String())
	for _, field := range []string{"attachment", "supporting_document"} {
		if key, uerr := utils.UploadFormFile(ctx, c, field, folder); uerr != nil {
			return utils.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to upload attachment: %v", uerr))
		} else if key != nil {
			req.SupportingDocument = key
			break
		}
	}

	correction, err := h.service.Create(ctx, req)
	if err != nil {
		return utils.HandleError(c, err)
	}

	return utils.SuccessResponse(c, http.StatusCreated, "Attendance correction created successfully", correction)
}

// bindCreateAttendanceCorrectionRequest accepts both JSON and multipart payloads.
func bindCreateAttendanceCorrectionRequest(c *fiber.Ctx) (*model.CreateAttendanceCorrectionRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.CreateAttendanceCorrectionRequest{
			PersonnelID:    utils.FormUUID(c, "personnel_id"),
			CorrectionDate: utils.FormString(c, "correction_date"),
			CorrectionType: utils.FormString(c, "correction_type"),
			Reason:         utils.FormString(c, "reason"),
		}
		if v := utils.FormString(c, "checkin_time"); v != "" {
			req.CheckinTime = &v
		}
		if v := utils.FormString(c, "checkout_time"); v != "" {
			req.CheckoutTime = &v
		}
		if v := utils.FormString(c, "supporting_document"); v != "" {
			req.SupportingDocument = &v
		}
		return req, nil
	}

	var req model.CreateAttendanceCorrectionRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

// Update godoc
// @Summary Update attendance correction
// @Tags Attendance Corrections
// @Produce json
// @Param id path string true "Attendance Correction ID"
// @Param correction body model.UpdateAttendanceCorrectionRequest true "Attendance Correction data"
// @Success 200 {object} model.APIResponse{data=model.AttendanceCorrection}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendance-corrections/{id} [put]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid attendance correction ID format")
	}

	var req model.UpdateAttendanceCorrectionRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
	}

	correction, err := h.service.Update(ctx, id, &req)
	if err != nil {
		return utils.HandleError(c, err)
	}

	return utils.SuccessResponse(c, http.StatusOK, "Attendance correction updated successfully", correction)
}

// Delete godoc
// @Summary Delete attendance correction
// @Tags Attendance Corrections
// @Produce json
// @Param id path string true "Attendance Correction ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendance-corrections/{id} [delete]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid attendance correction ID format")
	}

	if err := h.service.Delete(ctx, id); err != nil {
		return utils.HandleError(c, err)
	}

	return utils.SuccessResponse(c, http.StatusOK, "Attendance correction deleted successfully", nil)
}

// Pending godoc
// @Summary Get pending attendance corrections awaiting approval
// @Tags Attendance Corrections
// @Produce json
// @Success 200 {object} model.APIResponse
// @Router /attendance-corrections/pending [get]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Pending(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	page, limit = utils.ValidatePagination(page, limit)

	items, total, err := h.service.GetPending(ctx, page, limit)
	if err != nil {
		return utils.HandleError(c, err)
	}

	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "Pending attendance corrections retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

// Approve godoc
// @Summary Approve an attendance correction request
// @Tags Attendance Corrections
// @Accept json
// @Produce json
// @Param id path string true "Attendance Correction ID"
// @Param body body model.ApproveAttendanceCorrectionRequest true "Approval payload"
// @Success 200 {object} model.APIResponse
// @Router /attendance-corrections/{id}/approve [post]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Approve(c *fiber.Ctx) error {
	return h.processApproval(c, "approved")
}

// Reject godoc
// @Summary Reject an attendance correction request
// @Tags Attendance Corrections
// @Accept json
// @Produce json
// @Param id path string true "Attendance Correction ID"
// @Param body body model.ApproveAttendanceCorrectionRequest true "Rejection payload"
// @Success 200 {object} model.APIResponse
// @Router /attendance-corrections/{id}/reject [post]
// @Security BearerAuth
func (h *AttendanceCorrectionHandler) Reject(c *fiber.Ctx) error {
	return h.processApproval(c, "rejected")
}

func (h *AttendanceCorrectionHandler) processApproval(c *fiber.Ctx, defaultStatus string) error {
	ctx, cancel := context.WithTimeout(c.Context(), defaultTimeout)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "Invalid attendance correction ID format")
	}

	var req model.ApproveAttendanceCorrectionRequest
	_ = c.BodyParser(&req)
	if req.Status == "" {
		req.Status = defaultStatus
	}
	if req.Status != "approved" && req.Status != "rejected" {
		return utils.ErrorResponse(c, http.StatusBadRequest, "status must be 'approved' or 'rejected'")
	}

	approverID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || approverID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "Unauthorized")
	}

	notes := ""
	if req.ApproverNotes != nil {
		notes = *req.ApproverNotes
	}

	correction, err := h.service.Approve(ctx, id, approverID, req.Status, notes)
	if err != nil {
		return utils.HandleError(c, err)
	}

	return utils.SuccessResponse(c, http.StatusOK, "Attendance correction "+req.Status+" successfully", correction)
}
