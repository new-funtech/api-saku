package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func coerceFloatFields(body []byte, fields ...string) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	changed := false
	for _, f := range fields {
		raw, ok := m[f]
		if !ok || len(raw) == 0 {
			continue
		}
		if raw[0] != '"' {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			m[f] = json.RawMessage("null")
			changed = true
			continue
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			continue
		}
		m[f] = json.RawMessage(s)
		changed = true
	}
	if !changed {
		return body
	}
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

func coerceNumberToStringFields(body []byte, fields ...string) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	changed := false
	for _, f := range fields {
		raw, ok := m[f]
		if !ok || len(raw) == 0 {
			continue
		}
		s := strings.TrimSpace(string(raw))
		if s == "null" {
			continue
		}
		if strings.HasPrefix(s, "\"") {
			// Already a string; ensure non-empty becomes null too.
			var cur string
			if err := json.Unmarshal(raw, &cur); err == nil && strings.TrimSpace(cur) == "" {
				m[f] = json.RawMessage("null")
				changed = true
			}
			continue
		}
		// Validate it's a number, then quote it.
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			continue
		}
		quoted, err := json.Marshal(s)
		if err != nil {
			continue
		}
		m[f] = quoted
		changed = true
	}
	if !changed {
		return body
	}
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

type AttendanceHandler struct {
	service services.AttendanceService
}

func NewAttendanceHandler(service services.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{
		service: service,
	}
}

// GetAll godoc
// @Summary Get all attendances
// @Tags Attendances
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param personnel_id query string false "Filter by personnel ID"
// @Param location_id query string false "Filter by location ID"
// @Param status query string false "Filter by status"
// @Success 200 {object} model.APIResponse{data=[]model.Attendance,meta=model.PaginationMeta}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendances [get]
// @Security BearerAuth
func (h *AttendanceHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	personnelID := c.Query("personnel_id")
	locationID := c.Query("location_id")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	filters := make(map[string]interface{})
	if personnelID != "" {
		filters["personnel_id"] = personnelID
	}
	if locationID != "" {
		filters["location_id"] = locationID
	}
	if status != "" {
		filters["status"] = status
	}

	utils.ApplyBujpScope(c, filters)

	attendances, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to retrieve attendances",
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	hasNext := int64(page) < totalPages
	hasPrevious := page > 1

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Attendances retrieved successfully",
		Data:    attendances,
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
// @Summary Get attendance by ID
// @Tags Attendances
// @Produce json
// @Param id path string true "Attendance ID"
// @Success 200 {object} model.APIResponse{data=model.Attendance}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendances/{id} [get]
// @Security BearerAuth
func (h *AttendanceHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid attendance ID",
		})
	}

	attendance, err := h.service.GetByID(ctx, id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Attendance not found",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Attendance retrieved successfully",
		Data:    attendance,
	})
}

// Create godoc
// @Summary Create new attendance
// @Tags Attendances
// @Produce json
// @Param attendance body model.CreateAttendanceRequest true "Attendance data"
// @Success 201 {object} model.APIResponse{data=model.Attendance}
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendances [post]
// @Security BearerAuth
func (h *AttendanceHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := bindCreateAttendanceRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("ATTENDANCES/%s", uid.String())
	if key, uerr := utils.UploadFormFile(ctx, c, "check_in_photo", folder); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload check_in_photo: %v", uerr),
		})
	} else if key != nil {
		req.CheckInPhoto = key
	}
	if key, uerr := utils.UploadFormFile(ctx, c, "check_out_photo", folder); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload check_out_photo: %v", uerr),
		})
	} else if key != nil {
		req.CheckOutPhoto = key
	}
	for _, field := range []string{"supporting_document", "attachment", "document"} {
		key, uerr := utils.UploadFormFile(ctx, c, field, folder)
		if uerr != nil {
			return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
				Status: "error", Code: http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to upload supporting document: %v", uerr),
			})
		}
		if key != nil {
			req.SupportingDocument = key
			break
		}
	}

	if err := persistBase64AttendanceMedia(ctx, folder, &req.CheckInPhoto, &req.CheckOutPhoto, &req.SupportingDocument); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to persist attendance media: %v", err),
		})
	}

	attendance, err := h.service.CreateForUser(ctx, uid, req)
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
		Message: "Attendance created successfully",
		Data:    attendance,
	})
}

// bindCreateAttendanceRequest accepts both JSON and multipart payloads.
func bindCreateAttendanceRequest(c *fiber.Ctx) (*model.CreateAttendanceRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.CreateAttendanceRequest{
			PersonnelID: utils.FormUUID(c, "personnel_id"),
			LocationID:  utils.FormUUID(c, "location_id"),
			Date:        utils.FormString(c, "date"),
			Status:      utils.FormString(c, "status"),
		}
		if aid := utils.FormUUID(c, "assignment_id"); aid != uuid.Nil {
			req.AssignmentID = &aid
		}
		if v := utils.FormString(c, "check_in"); v != "" {
			req.CheckIn = &v
		}
		if v := utils.FormString(c, "check_out"); v != "" {
			req.CheckOut = &v
		}
		if v := strings.TrimSpace(c.FormValue("check_in_latitude")); v != "" {
			req.CheckInLatitude = model.NumberString(v)
		}
		if v := strings.TrimSpace(c.FormValue("check_in_longitude")); v != "" {
			req.CheckInLongitude = model.NumberString(v)
		}
		if v := strings.TrimSpace(c.FormValue("check_out_latitude")); v != "" {
			req.CheckOutLatitude = model.NumberString(v)
		}
		if v := strings.TrimSpace(c.FormValue("check_out_longitude")); v != "" {
			req.CheckOutLongitude = model.NumberString(v)
		}
		if v := utils.FormString(c, "notes"); v != "" {
			req.Notes = &v
		}
		if v := utils.FormString(c, "check_in_photo"); v != "" {
			req.CheckInPhoto = &v
		}
		if v := utils.FormString(c, "check_out_photo"); v != "" {
			req.CheckOutPhoto = &v
		}
		if v := utils.FormString(c, "supporting_document"); v != "" {
			req.SupportingDocument = &v
		}
		return req, nil
	}

	body := coerceNumberToStringFields(c.Body(),
		"check_in_latitude", "check_in_longitude",
		"check_out_latitude", "check_out_longitude",
	)
	var req model.CreateAttendanceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// Update godoc
// @Summary Update attendance
// @Tags Attendances
// @Produce json
// @Param id path string true "Attendance ID"
// @Param attendance body model.UpdateAttendanceRequest true "Attendance data"
// @Success 200 {object} model.APIResponse{data=model.Attendance}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendances/{id} [put]
// @Security BearerAuth
func (h *AttendanceHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid attendance ID",
		})
	}

	req, err := bindUpdateAttendanceRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("ATTENDANCES/%s", uid.String())
	if key, uerr := utils.UploadFormFile(ctx, c, "check_out_photo", folder); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload check_out_photo: %v", uerr),
		})
	} else if key != nil {
		req.CheckOutPhoto = key
	}
	for _, field := range []string{"supporting_document", "attachment", "document"} {
		key, uerr := utils.UploadFormFile(ctx, c, field, folder)
		if uerr != nil {
			return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
				Status: "error", Code: http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to upload supporting document: %v", uerr),
			})
		}
		if key != nil {
			req.SupportingDocument = key
			break
		}
	}

	if err := persistBase64AttendanceMedia(ctx, folder, nil, &req.CheckOutPhoto, &req.SupportingDocument); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to persist attendance media: %v", err),
		})
	}

	attendance, err := h.service.Update(ctx, id, req)
	if err != nil {
		status := statusForServiceError(err)
		return c.Status(status).JSON(model.APIResponse{
			Status:  "error",
			Code:    status,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Attendance updated successfully",
		Data:    attendance,
	})
}

// Checkout godoc
// @Summary Check out for the current authenticated user (today's attendance)
// @Tags Attendances
// @Produce json
// @Success 200 {object} model.APIResponse
// @Router /attendances/checkout [put]
// @Security BearerAuth
func (h *AttendanceHandler) Checkout(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := bindUpdateAttendanceRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	uid, _ := c.Locals("userID").(uuid.UUID)
	folder := fmt.Sprintf("ATTENDANCES/%s", uid.String())
	if key, uerr := utils.UploadFormFile(ctx, c, "check_out_photo", folder); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload check_out_photo: %v", uerr),
		})
	} else if key != nil {
		req.CheckOutPhoto = key
	}
	for _, field := range []string{"supporting_document", "attachment", "document"} {
		key, uerr := utils.UploadFormFile(ctx, c, field, folder)
		if uerr != nil {
			return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
				Status: "error", Code: http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to upload supporting document: %v", uerr),
			})
		}
		if key != nil {
			req.SupportingDocument = key
			break
		}
	}

	if err := persistBase64AttendanceMedia(ctx, folder, nil, &req.CheckOutPhoto, &req.SupportingDocument); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status: "error", Code: http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to persist attendance media: %v", err),
		})
	}

	attendance, err := h.service.CheckoutForUser(ctx, uid, req)
	if err != nil {
		status := statusForServiceError(err)
		return c.Status(status).JSON(model.APIResponse{
			Status:  "error",
			Code:    status,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Checked out successfully",
		Data:    attendance,
	})
}

func bindUpdateAttendanceRequest(c *fiber.Ctx) (*model.UpdateAttendanceRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		req := &model.UpdateAttendanceRequest{}
		if v := utils.FormString(c, "check_out"); v != "" {
			req.CheckOut = &v
		}
		if v := strings.TrimSpace(c.FormValue("check_out_latitude")); v != "" {
			req.CheckOutLatitude = model.NumberString(v)
		}
		if v := strings.TrimSpace(c.FormValue("check_out_longitude")); v != "" {
			req.CheckOutLongitude = model.NumberString(v)
		}
		if v := utils.FormString(c, "check_out_photo"); v != "" {
			req.CheckOutPhoto = &v
		}
		if v := utils.FormString(c, "supporting_document"); v != "" {
			req.SupportingDocument = &v
		}
		if v := utils.FormString(c, "status"); v != "" {
			req.Status = &v
		}
		if v := utils.FormString(c, "notes"); v != "" {
			req.Notes = &v
		}
		if v := strings.TrimSpace(c.FormValue("work_duration")); v != "" {
			n := utils.FormInt(c, "work_duration", 0)
			req.WorkDuration = &n
		}
		return req, nil
	}

	body := coerceNumberToStringFields(c.Body(),
		"check_out_latitude", "check_out_longitude",
	)
	var req model.UpdateAttendanceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func persistBase64AttendanceMedia(ctx context.Context, folder string, fields ...**string) error {
	for _, fp := range fields {
		if fp == nil || *fp == nil {
			continue
		}
		val := **fp
		if val == "" {
			continue
		}
		key, err := utils.UploadBase64DataURL(ctx, val, folder)
		if err != nil {
			return err
		}
		if key != "" && key != val {
			**fp = key
		}
	}
	return nil
}

// Delete godoc
// @Summary Delete attendance
// @Tags Attendances
// @Produce json
// @Param id path string true "Attendance ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /attendances/{id} [delete]
// @Security BearerAuth
func (h *AttendanceHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Invalid attendance ID",
		})
	}

	if err := h.service.Delete(ctx, id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete attendance",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Attendance deleted successfully",
	})
}
