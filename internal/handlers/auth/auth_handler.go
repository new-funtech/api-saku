package auth

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	authSvc "github.com/ganiramadhan/ganipedia/backend/internal/services/auth"
	"github.com/gofiber/fiber/v2"
)

// Handler handles authentication HTTP requests
type Handler struct {
	service authSvc.Service
}

// NewHandler creates a new auth handler
func NewHandler(service authSvc.Service) *Handler {
	return &Handler{service: service}
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user with email and password, returns JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      model.LoginRequest  true  "Login credentials"
// @Success      200      {object}  model.APIResponse{data=model.AuthResponse}
// @Failure      400      {object}  model.APIResponse
// @Failure      401      {object}  model.APIResponse
// @Router       /api/v1/auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	result, err := h.service.Login(req)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusUnauthorized,
			Message: constants.ErrInvalidCredentials,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessLogin,
		Data:    result,
	})
}

// Register godoc
// @Summary      User registration
// @Description  Register a new user account and returns JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      model.RegisterRequest  true  "Registration data"
// @Success      201      {object}  model.APIResponse{data=model.AuthResponse}
// @Failure      400      {object}  model.APIResponse
// @Failure      409      {object}  model.APIResponse
// @Router       /api/v1/auth/register [post]
func (h *Handler) Register(c *fiber.Ctx) error {
	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	result, err := h.service.Register(req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := constants.ErrInternalServer
		if err.Error() == "email already exists" {
			statusCode = http.StatusConflict
			message = constants.ErrEmailExists
		}
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: message,
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: constants.SuccessRegister,
		Data:    result,
	})
}
