package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/timeout"
)

func TimeoutMiddleware(duration time.Duration) fiber.Handler {
	return timeout.NewWithContext(func(c *fiber.Ctx) error {
		return c.Next()
	}, duration)
}

const DefaultTimeout = 30 * time.Second
