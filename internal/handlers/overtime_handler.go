package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type OvertimeHandler struct {
	service services.OvertimeService
}

func NewOvertimeHandler(service services.OvertimeService) *OvertimeHandler {
	return &OvertimeHandler{service: service}
}

func isStaffRole(c *fiber.Ctx) bool {
	role, _ := c.Locals("role").(string)
	return role == "guard" || role == "supervisor"
}

func callerPersonnelID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals("personnelID").(uuid.UUID)
	return id
}

func overtimeError(c *fiber.Ctx, err error) error {
	status := statusForServiceError(err)
	return c.Status(status).JSON(model.APIResponse{Status: "error", Code: status, Message: err.Error()})
}

// loadAccessible loads an overtime request the caller may see: staff their own, admins within their BUJP.
func (h *OvertimeHandler) loadAccessible(c *fiber.Ctx, ctx context.Context) (*model.OvertimeResponse, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return nil, c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "Invalid overtime ID"})
	}
	o, err := h.service.GetByID(ctx, id)
	if err != nil {
		return nil, c.Status(http.StatusNotFound).JSON(model.APIResponse{Status: "error", Code: http.StatusNotFound, Message: "Overtime not found"})
	}
	if isStaffRole(c) {
		if pid := callerPersonnelID(c); pid == uuid.Nil || pid != o.PersonnelID {
			return nil, forbiddenResponse(c)
		}
	} else if o.Personnel != nil && !utils.CanAccessBujp(c, o.Personnel.BujpID) {
		return nil, forbiddenResponse(c)
	}
	return o, nil
}

// GetAll lists overtime requests: staff see their own, admins everyone in their scope.
func (h *OvertimeHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, limit := utils.ParsePagination(c)

	filters := map[string]interface{}{}
	if pid, err := uuid.Parse(c.Query("personnel_id")); err == nil {
		filters["personnel_id"] = pid
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if d, err := time.ParseInLocation("2006-01-02", c.Query("date_from"), time.Local); err == nil {
		filters["date_from"] = d
	}
	if d, err := time.ParseInLocation("2006-01-02", c.Query("date_to"), time.Local); err == nil {
		filters["date_to"] = d
	}
	utils.ApplyBujpScope(c, filters)
	if isStaffRole(c) {
		if pid := callerPersonnelID(c); pid == uuid.Nil {
			filters["force_empty"] = true
		} else {
			filters["personnel_id"] = pid
		}
	}

	items, total, err := h.service.GetAll(ctx, page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Failed to retrieve overtime requests"})
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status: "success", Code: http.StatusOK, Message: "Overtime requests retrieved", Data: items,
		Meta: &model.PaginationMeta{
			Page: page, Limit: limit, Total: total, TotalPages: int(totalPages),
			HasNext: int64(page) < totalPages, HasPrevious: page > 1,
		},
	})
}

// Pending lists requests awaiting a decision (admin roles only; see routes).
func (h *OvertimeHandler) Pending(c *fiber.Ctx) error {
	c.Request().URI().QueryArgs().Set("status", "pending")
	return h.GetAll(c)
}

func (h *OvertimeHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	o, err := h.loadAccessible(c, ctx)
	if o == nil {
		return err
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Overtime retrieved", Data: o})
}

// Create files a request for the caller; staff cannot file for someone else.
func (h *OvertimeHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var req model.CreateOvertimeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "Invalid request body"})
	}
	req.PersonnelID = callerPersonnelID(c)
	if req.PersonnelID == uuid.Nil {
		return c.Status(http.StatusForbidden).JSON(model.APIResponse{Status: "error", Code: http.StatusForbidden, Message: "Akun Anda tidak terhubung dengan data personel"})
	}
	o, err := h.service.Create(ctx, &req)
	if err != nil {
		return overtimeError(c, err)
	}
	return c.Status(http.StatusCreated).JSON(model.APIResponse{Status: "success", Code: http.StatusCreated, Message: "Overtime created", Data: o})
}

func (h *OvertimeHandler) Update(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	existing, err := h.loadAccessible(c, ctx)
	if existing == nil {
		return err
	}
	var req model.UpdateOvertimeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "Invalid request body"})
	}
	o, err := h.service.Update(ctx, existing.ID, &req)
	if err != nil {
		return overtimeError(c, err)
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Overtime updated", Data: o})
}

func (h *OvertimeHandler) Delete(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	existing, err := h.loadAccessible(c, ctx)
	if existing == nil {
		return err
	}
	if err := h.service.Delete(ctx, existing.ID); err != nil {
		return overtimeError(c, err)
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Overtime deleted"})
}

func (h *OvertimeHandler) Approve(c *fiber.Ctx) error { return h.decide(c, "approved") }
func (h *OvertimeHandler) Reject(c *fiber.Ctx) error  { return h.decide(c, "rejected") }

func (h *OvertimeHandler) decide(c *fiber.Ctx, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	existing, err := h.loadAccessible(c, ctx)
	if existing == nil {
		return err
	}
	var req model.DecideOvertimeRequest
	_ = c.BodyParser(&req)
	notes := ""
	if req.ApproverNotes != nil {
		notes = *req.ApproverNotes
	}
	approverID, _ := c.Locals("userID").(uuid.UUID)
	if approverID == uuid.Nil {
		return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{Status: "error", Code: http.StatusUnauthorized, Message: "Unauthorized"})
	}
	o, err := h.service.Decide(ctx, existing.ID, approverID, status, notes)
	if err != nil {
		return overtimeError(c, err)
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Overtime " + status, Data: o})
}
