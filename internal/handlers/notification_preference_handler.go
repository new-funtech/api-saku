package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type NotificationPreferenceHandler struct {
	repo repository.NotificationPreferenceRepository
}

func NewNotificationPreferenceHandler(repo repository.NotificationPreferenceRepository) *NotificationPreferenceHandler {
	return &NotificationPreferenceHandler{repo: repo}
}

// Get godoc
// @Summary      The user's push and email notification switches
// @Tags         Notifications
// @Produce      json
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/preferences [get]
func (h *NotificationPreferenceHandler) Get(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	push, email, err := h.repo.Get(c.Context(), userID)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "failed to load preferences")
	}
	return utils.SuccessResponse(c, http.StatusOK, "preferences retrieved", model.NotificationPreferenceResponse{PushEnabled: push, EmailEnabled: email})
}

// Update godoc
// @Summary      Turn push and/or email notifications on or off
// @Tags         Notifications
// @Accept       json
// @Produce      json
// @Param        body body model.NotificationPreferenceRequest true "Switches"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /notifications/preferences [put]
func (h *NotificationPreferenceHandler) Update(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	var req model.NotificationPreferenceRequest
	if err := c.BodyParser(&req); err != nil || (req.PushEnabled == nil && req.EmailEnabled == nil) {
		return utils.ErrorResponse(c, http.StatusBadRequest, "push_enabled or email_enabled is required")
	}
	push, email, err := h.repo.Get(c.Context(), userID)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "failed to load preferences")
	}
	if req.PushEnabled != nil {
		push = *req.PushEnabled
	}
	if req.EmailEnabled != nil {
		email = *req.EmailEnabled
	}
	if err := h.repo.Upsert(c.Context(), userID, push, email); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "failed to save preferences")
	}
	return utils.SuccessResponse(c, http.StatusOK, "preferences saved", model.NotificationPreferenceResponse{PushEnabled: push, EmailEnabled: email})
}
