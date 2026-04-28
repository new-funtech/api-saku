package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	service services.AuthService
}

// NewHandler creates a new auth handler
func NewAuthHandler(service services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
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
// @Router       /api/v1/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req model.LoginRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}

	result, err := h.service.Login(c.Context(), req)
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
// @Router       /api/v1/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req model.RegisterRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}

	result, err := h.service.Register(c.Context(), req)
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
