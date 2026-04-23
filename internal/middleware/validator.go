package middleware

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

func ValidateRequest(data interface{}) error {
	return validate.Struct(data)
}

func ValidationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

func ValidateAndParse(c *fiber.Ctx, data interface{}) error {
	if err := c.BodyParser(data); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	if err := ValidateRequest(data); err != nil {
		validationErrors := make(map[string]string)
		if validErrs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range validErrs {
				field := e.Field()
				switch e.Tag() {
				case "required":
					validationErrors[field] = field + " is required"
				case "email":
					validationErrors[field] = field + " must be a valid email"
				case "min":
					validationErrors[field] = field + " must be at least " + e.Param() + " characters"
				case "gt":
					validationErrors[field] = field + " must be greater than " + e.Param()
				case "gte":
					validationErrors[field] = field + " must be greater than or equal to " + e.Param()
				default:
					validationErrors[field] = field + " is invalid"
				}
			}
		}

		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Validation failed",
			Data:    validationErrors,
		})
	}

	return nil
}
