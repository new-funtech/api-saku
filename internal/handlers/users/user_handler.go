package users

import (
	"net/http"
	"strconv"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	usersvc "github.com/ganiramadhan/ganipedia/backend/internal/services/user"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UserHandler struct {
	service usersvc.Service
}

func NewUserHandler(service usersvc.Service) *UserHandler {
	return &UserHandler{service: service}
}

// GetAllUsers godoc
// @Summary      Get all users
// @Description  Retrieve a paginated list of all users sorted by newest first
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        page   query     int  false  "Page number" default(1)
// @Param        limit  query     int  false  "Items per page" default(10)
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/users [get]
func (h *UserHandler) GetAllUsers(c *fiber.Ctx) error {
	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	users, meta, err := h.service.GetAllUsers(c.Context(), page, limit)
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

	user, err := h.service.CreateUser(c.Context(), req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := constants.ErrInternalServer
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

	err = h.service.DeleteUser(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrUserNotFound,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessDeleteUser,
	})
}
