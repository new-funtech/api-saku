package middleware

import (
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

func extractToken(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return header
}

func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
				Status:  "error",
				Code:    http.StatusUnauthorized,
				Message: constants.ErrUnauthorized,
			})
		}

		tokenString := extractToken(authHeader)
		claims, err := jwtutil.ValidateToken(tokenString)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(model.APIResponse{
				Status:  "error",
				Code:    http.StatusUnauthorized,
				Message: constants.ErrInvalidToken,
			})
		}

		// Set user info in Fiber context
		c.Locals("userID", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}
