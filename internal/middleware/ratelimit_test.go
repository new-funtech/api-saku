package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimitByIPAnswersWithRetryAfter(t *testing.T) {
	app := fiber.New()
	app.Use(RateLimitByIP(2, time.Minute))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 2; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("request %d: status %v err %v", i, resp, err)
		}
	}
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Terlalu banyak permintaan") {
		t.Errorf("unexpected body: %s", body)
	}
}
