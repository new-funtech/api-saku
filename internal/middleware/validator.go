package middleware

import (
	"errors"
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

// ErrResponseWritten is returned by ValidateAndParse after it has already written the 400 answer.
// Handlers just `return err`; the app's error handler recognises it and leaves the response alone.
// (It used to return the result of writing the response, which is nil, so a failed validation was
// answered and then ignored: the handler went on and processed the invalid request anyway.)
var ErrResponseWritten = errors.New("validation response already written")

// ValidateAndParse binds and validates the body. On failure it writes the 400 response and returns
// ErrResponseWritten, so the caller must stop: `if err := ...; err != nil { return err }`.
func ValidateAndParse(c *fiber.Ctx, data interface{}) error {
	if err := c.BodyParser(data); err != nil {
		if werr := c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		}); werr != nil {
			return werr
		}
		return ErrResponseWritten
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

		if werr := c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Validation failed",
			Data:    validationErrors,
		}); werr != nil {
			return werr
		}
		return ErrResponseWritten
	}

	return nil
}
