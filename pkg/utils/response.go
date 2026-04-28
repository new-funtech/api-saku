package utils

import (
	"github.com/ganiramadhan/ganipedia/backend/internal/errors"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	return c.Status(statusCode).JSON(model.APIResponse{
		Status:  "success",
		Code:    statusCode,
		Message: message,
		Data:    data,
	})
}

func SuccessResponseWithMeta(c *fiber.Ctx, statusCode int, message string, data interface{}, meta *model.PaginationMeta) error {
	return c.Status(statusCode).JSON(model.APIResponse{
		Status:  "success",
		Code:    statusCode,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

func ErrorResponse(c *fiber.Ctx, statusCode int, message string, data ...interface{}) error {
	response := model.APIResponse{
		Status:  "error",
		Code:    statusCode,
		Message: message,
	}

	if len(data) > 0 {
		response.Data = data[0]
	}

	return c.Status(statusCode).JSON(response)
}

func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return ErrorResponse(c, fiber.StatusInternalServerError, "Unknown error occurred")
	}

	if appErr, ok := err.(*errors.AppError); ok {
		return ErrorResponse(c, appErr.StatusCode, appErr.Message, appErr.Context)
	}

	statusCode := errors.GetStatusCode(err)
	message := err.Error()

	return ErrorResponse(c, statusCode, message)
}
