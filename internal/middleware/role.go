package middleware

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

// RequireRoles lets only the listed roles through. It must run after
// AuthRequired, which puts the caller's role in the request locals.
func RequireRoles(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if _, ok := allowed[role]; !ok {
			return c.Status(http.StatusForbidden).JSON(model.APIResponse{
				Status:  "error",
				Code:    http.StatusForbidden,
				Message: "Anda tidak memiliki akses untuk tindakan ini",
			})
		}
		return c.Next()
	}
}
