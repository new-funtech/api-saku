package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	service services.NotificationService
}

func NewNotificationHandler(service services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

// GetAll godoc
// @Summary      List the authenticated user's notifications
// @Tags         Notifications
// @Produce      json
// @Param        page  query int false "Page number"
// @Param        limit query int false "Items per page"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications [get]
func (h *NotificationHandler) GetAll(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	page, limit := utils.ParsePagination(c)

	items, total, err := h.service.List(c.Context(), userID, page, limit)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "notifications retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

// GetUnreadCount godoc
// @Summary      Get the authenticated user's unread notification count
// @Tags         Notifications
// @Produce      json
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/unread-count [get]
func (h *NotificationHandler) GetUnreadCount(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	count, err := h.service.UnreadCount(c.Context(), userID)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "unread count retrieved", fiber.Map{"count": count})
}

// MarkAsRead godoc
// @Summary      Mark one notification as read
// @Tags         Notifications
// @Produce      json
// @Param        id path string true "Notification UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/{id}/read [post]
func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid notification id")
	}
	if err := h.service.MarkRead(c.Context(), userID, id); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "notification marked as read", nil)
}

// MarkAllAsRead godoc
// @Summary      Mark all of the authenticated user's notifications as read
// @Tags         Notifications
// @Produce      json
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/read-all [post]
func (h *NotificationHandler) MarkAllAsRead(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	if err := h.service.MarkAllRead(c.Context(), userID); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "all notifications marked as read", nil)
}

// Delete godoc
// @Summary      Delete one notification
// @Tags         Notifications
// @Produce      json
// @Param        id path string true "Notification UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/{id} [delete]
func (h *NotificationHandler) Delete(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid notification id")
	}
	if err := h.service.Delete(c.Context(), userID, id); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "notification deleted", nil)
}

// ClearRead godoc
// @Summary      Delete all of the authenticated user's already-read notifications
// @Tags         Notifications
// @Produce      json
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/clear-read [delete]
func (h *NotificationHandler) ClearRead(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	if err := h.service.ClearRead(c.Context(), userID); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "read notifications cleared", nil)
}
