package handlers

import (
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DeviceHandler struct {
	repo repository.DeviceTokenRepository
}

func NewDeviceHandler(repo repository.DeviceTokenRepository) *DeviceHandler {
	return &DeviceHandler{repo: repo}
}

// RegisterToken godoc
// @Summary      Register the device's FCM token
// @Description  Upserts by token: a token already used by another user moves to the signed-in user.
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        body body model.RegisterDeviceTokenRequest true "Token"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /devices/token [post]
func (h *DeviceHandler) RegisterToken(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	var req model.RegisterDeviceTokenRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "android"
	}
	if err := h.repo.Upsert(c.Context(), userID, strings.TrimSpace(req.FCMToken), platform, req.DeviceID); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "failed to save device token")
	}
	return utils.SuccessResponse(c, http.StatusOK, "device token saved", nil)
}

// UnregisterToken godoc
// @Summary      Remove the device's FCM token (sign-out)
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        body body model.UnregisterDeviceTokenRequest true "Token"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /devices/token [delete]
func (h *DeviceHandler) UnregisterToken(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return utils.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
	}
	var req model.UnregisterDeviceTokenRequest
	// DELETE bodies are not always sent by HTTP clients, so the token may also come as a query value.
	if q := c.Query("fcm_token"); q != "" {
		req.FCMToken = q
	} else if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}
	if len(req.FCMToken) < 20 {
		return utils.ErrorResponse(c, http.StatusBadRequest, "fcm_token is required")
	}
	// Removing a token that is not there (or not yours) is not an error: sign-out must always succeed.
	if _, err := h.repo.DeleteForUser(c.Context(), userID, strings.TrimSpace(req.FCMToken)); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "failed to remove device token")
	}
	return utils.SuccessResponse(c, http.StatusOK, "device token removed", nil)
}
