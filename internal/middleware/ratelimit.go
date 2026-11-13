package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
)

// keyByIP limits per client address (used before the caller is known, e.g. login).
func keyByIP(c *fiber.Ctx) string { return "ip:" + c.IP() + ":" + c.Path() }

// keyByUser limits per signed-in user, so many people behind one mobile-carrier address do not
// share a budget. It falls back to the address when the user is not known yet.
func keyByUser(c *fiber.Ctx) string {
	if id, ok := c.Locals("userID").(uuid.UUID); ok && id != uuid.Nil {
		return "user:" + id.String()
	}
	return "ip:" + c.IP()
}

// limitReached answers 429 with a message the apps can show as is, and how long to wait.
func limitReached(c *fiber.Ctx) error {
	wait, _ := strconv.Atoi(c.GetRespHeader(fiber.HeaderRetryAfter))
	if wait <= 0 {
		wait = 60
	}
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(wait))
	return c.Status(http.StatusTooManyRequests).JSON(model.APIResponse{
		Status:  "error",
		Code:    http.StatusTooManyRequests,
		Message: "Terlalu banyak percobaan. Silakan coba lagi dalam " + humanWait(wait) + ".",
	})
}

func newLimiter(max int, window time.Duration, key func(*fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   window,
		KeyGenerator: key,
		LimitReached: limitReached,
	})
}

// RateLimitByIP caps requests per address and path.
func RateLimitByIP(max int, window time.Duration) fiber.Handler {
	return newLimiter(max, window, keyByIP)
}

// RateLimitByUser caps requests per signed-in user. Mount it after AuthRequired.
func RateLimitByUser(max int, window time.Duration) fiber.Handler {
	return newLimiter(max, window, keyByUser)
}

// humanWait renders seconds as "45 detik", "2 menit" or "2 menit 10 detik".
func humanWait(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d detik", seconds)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d menit", seconds/60)
	}
	return fmt.Sprintf("%d menit %d detik", seconds/60, seconds%60)
}
