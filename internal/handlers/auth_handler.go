package handlers

import (
	"errors"
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	service services.AuthService
}

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

// ForgotPassword godoc
// @Summary      Forgot password
// @Description  Mengirim OTP 6-digit ke email user untuk reset password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      model.ForgotPasswordRequest  true  "Email akun"
// @Success      200      {object}  model.APIResponse
// @Router       /api/v1/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req model.ForgotPasswordRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}
	if err := h.service.ForgotPassword(c.Context(), req); err != nil {
		switch {
		case errors.Is(err, services.ErrEmailNotRegistered):
			return c.Status(http.StatusNotFound).JSON(model.APIResponse{
				Status: "error", Code: http.StatusNotFound,
				Message: constants.ErrEmailNotRegistered,
			})
		case errors.Is(err, services.ErrEmailQueueFailed):
			return c.Status(http.StatusServiceUnavailable).JSON(model.APIResponse{
				Status: "error", Code: http.StatusServiceUnavailable,
				Message: constants.ErrEmailQueueFailed,
			})
		default:
			return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
				Status: "error", Code: http.StatusInternalServerError,
				Message: constants.ErrInternalServer,
			})
		}
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status: "success", Code: http.StatusOK,
		Message: constants.SuccessForgotPasswordSent,
	})
}

// VerifyResetOTP godoc
// @Summary      Verify reset OTP
// @Description  Cek OTP yang dikirim ke email, mengembalikan reset_token sementara
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      model.VerifyResetOTPRequest  true  "Email + OTP"
// @Success      200      {object}  model.APIResponse{data=model.VerifyResetOTPResponse}
// @Failure      400      {object}  model.APIResponse
// @Router       /api/v1/verify-reset-otp [post]
func (h *AuthHandler) VerifyResetOTP(c *fiber.Ctx) error {
	var req model.VerifyResetOTPRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}
	resp, err := h.service.VerifyResetOTP(c.Context(), req)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest,
			Message: constants.ErrInvalidOTP,
		})
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status: "success", Code: http.StatusOK,
		Message: constants.SuccessVerifyResetOTP,
		Data:    resp,
	})
}

// ResetPassword godoc
// @Summary      Reset password
// @Description  Set password baru menggunakan reset_token yang valid
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      model.ResetPasswordRequest  true  "Reset token + password baru"
// @Success      200      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Router       /api/v1/reset-password [post]
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req model.ResetPasswordRequest
	if err := middleware.ValidateAndParse(c, &req); err != nil {
		return err
	}
	if err := h.service.ResetPassword(c.Context(), req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status: "error", Code: http.StatusBadRequest,
			Message: constants.ErrInvalidResetToken,
		})
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status: "success", Code: http.StatusOK,
		Message: constants.SuccessResetPassword,
	})
}
