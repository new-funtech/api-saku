package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UserHandler struct {
	service services.UserService
}

func NewUserHandler(service services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// GetAllUsers godoc
// @Summary      Get all users
// @Description  Retrieve a paginated list of all users sorted by newest first
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        page     query     int     false  "Page number" default(1)
// @Param        limit    query     int     false  "Items per page" default(10)
// @Param        search   query     string  false  "Search by name, email, or phone"
// @Param        role     query     string  false  "Filter by role"
// @Param        status   query     string  false  "Filter by status"
// @Param        bujp_id  query     string  false  "Filter by BUJP ID"
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users [get]
func (h *UserHandler) GetAllUsers(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)
	// Enforce maximum limit to prevent database overload
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}
	// Get current user ID to exclude from list
	userID := c.Locals("userID").(uuid.UUID)

	// Build filters from query params
	filters := make(map[string]interface{})
	filters["exclude_user_id"] = userID // Exclude logged-in user
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}
	if role := c.Query("role"); role != "" {
		filters["role"] = role
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if bujpID := c.Query("bujp_id"); bujpID != "" {
		if parsedID, err := uuid.Parse(bujpID); err == nil {
			filters["bujp_id"] = parsedID
		}
	}

	// Tenant scoping: BUJP-scoped callers (company_admin/supervisor) are
	// clamped to their own bujp; guards see only themselves; Pusat is
	// unaffected. This prevents cross-tenant user enumeration.
	utils.ApplyBujpScope(c, filters)

	users, meta, err := h.service.GetAllUsers(c.Context(), page, limit, filters)
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
		Message: constants.SuccessGetUsers,
		Data:    users,
		Meta:    meta,
	})
}

// GetUserByID godoc
// @Summary      Get user by ID
// @Description  Retrieve a single user by its UUID
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "User UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/{id} [get]
func (h *UserHandler) GetUserByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	user, err := h.service.GetUserByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrUserNotFound,
		})
	}

	if !utils.CanAccessOptionalBujp(c, user.BujpID) {
		return forbiddenResponse(c)
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessGetUser,
		Data:    user,
	})
}

// CreateUser godoc
// @Summary      Create a new user
// @Description  Create a new user with the provided data
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateUserRequest  true  "User data"
// @Success      201      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Failure      500      {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users [post]
func (h *UserHandler) CreateUser(c *fiber.Ctx) error {
	var req model.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	// Tenant scoping: a non-super_admin caller may only create users
	// belonging to their own BUJP. We force-override any BujpID supplied in
	// the payload to prevent privilege escalation across tenants.
	role, _ := c.Locals("role").(string)
	if role != "super_admin" {
		uid, _ := c.Locals("userID").(uuid.UUID)
		if caller, err := h.service.GetUserByID(c.Context(), uid); err == nil && caller != nil && caller.BujpID != nil {
			req.BujpID = caller.BujpID
		}
	}

	user, err := h.service.CreateUser(c.Context(), req)
	if err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: constants.SuccessCreateUser,
		Data:    user,
	})
}

// UpdateUser godoc
// @Summary      Update a user
// @Description  Update an existing user by its UUID
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id       path      string                   true  "User UUID"
// @Param        request  body      model.UpdateUserRequest  true  "User data"
// @Success      200      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Failure      404      {object}  model.APIResponse
// @Failure      500      {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/{id} [put]
func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	var req model.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	// Tenant ownership check: load the target user and confirm the caller
	// belongs to the same BUJP (Pusat bypasses).
	existing, err := h.service.GetUserByID(c.Context(), id)
	if err != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrUserNotFound,
		})
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
		return forbiddenResponse(c)
	}
	// Prevent BUJP-scoped callers from re-homing a user to another tenant.
	role, _ := c.Locals("role").(string)
	if !utils.IsPusat(role) {
		req.BujpID = existing.BujpID
	}

	user, err := h.service.UpdateUser(c.Context(), id, req)
	if err != nil {
		statusCode := http.StatusNotFound
		message := constants.ErrUserNotFound
		if err.Error() == "email already exists" {
			statusCode = http.StatusConflict
			message = "Email already exists"
		}
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: message,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessUpdateUser,
		Data:    user,
	})
}

// DeleteUser godoc
// @Summary      Delete a user
// @Description  Delete a user by its UUID
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "User UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	// Tenant ownership check before delete.
	existing, err := h.service.GetUserByID(c.Context(), id)
	if err != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrUserNotFound,
		})
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
		return forbiddenResponse(c)
	}

	err = h.service.DeleteUser(c.Context(), id)
	if err != nil {
		statusCode := statusForServiceError(err)
		message := err.Error()
		if statusCode == http.StatusInternalServerError {
			// Fall back to the legacy 404/not-found message so existing
			// callers keep their UX when the user simply does not exist.
			statusCode = http.StatusNotFound
			message = constants.ErrUserNotFound
		}
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: message,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessDeleteUser,
	})
}

// GetProfile godoc
// @Summary      Get authenticated user profile
// @Description  Retrieve the profile of the authenticated user with personnel and assignment data
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {object}  model.APIResponse
// @Failure      401  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/profile [get]
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	profile, err := h.service.GetProfile(c.Context(), userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to get profile",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Profile retrieved successfully",
		Data:    profile,
	})
}

// UpdateProfile godoc
// @Summary      Update authenticated user profile
// @Description  Update the profile of the authenticated user
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        request  body      model.UpdateProfileRequest  true  "Profile data"
// @Success      200      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Failure      500      {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/profile [put]
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var req model.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	profile, err := h.service.UpdateProfile(c.Context(), userID, req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Failed to update profile",
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Profile updated successfully",
		Data:    profile,
	})
}

// ChangePassword godoc
// @Summary      Change user password
// @Description  Change the current user's password
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        request  body      model.ChangePasswordRequest  true  "Password change data"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      401  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users/change-password [post]
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var req model.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
	}

	// Validate passwords match
	if req.NewPassword != req.ConfirmPassword {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "New password and confirm password do not match",
		})
	}

	if err := h.service.ChangePassword(c.Context(), userID, req); err != nil {
		if err.Error() == "current password is incorrect" {
			return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
				Status:  "error",
				Code:    http.StatusUnauthorized,
				Message: "Current password is incorrect",
			})
		}
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Password changed successfully",
	})
}
