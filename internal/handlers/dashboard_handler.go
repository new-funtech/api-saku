package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DashboardHandler struct {
	service services.DashboardService
}

func NewDashboardHandler(service services.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// GetSummary gets dashboard summary
// @Summary Get dashboard summary
// @Description Get comprehensive dashboard summary with statistics
// @Tags dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusUnauthorized,
			Message: constants.ErrUnauthorized,
		})
	}

	role, ok := c.Locals("role").(string)
	if !ok {
		role = "guard"
	}

	bujpID, _ := c.Locals("bujpID").(uuid.UUID)

	summary, err := h.service.GetSummary(c.Context(), userID, role, bujpID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrInternalServer,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Dashboard summary retrieved successfully",
		Data:    summary,
	})
}
