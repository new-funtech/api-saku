package middleware

import (
	"net/http"
	"testing"

	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestTemporaryPasswordTokenOnlyAllowsPasswordChange(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := jwtutil.GenerateToken(uuid.New(), "guard@example.com", "guard", true)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Use(AuthRequired())
	app.Get("/api/v1/users/profile", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/api/v1/users/change-password", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	profileRequest, err := http.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	if err != nil {
		t.Fatal(err)
	}
	profileRequest.Header.Set("Authorization", "Bearer "+token)
	profileResponse, err := app.Test(profileRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer profileResponse.Body.Close()
	if profileResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("profile status = %d, want %d", profileResponse.StatusCode, http.StatusForbidden)
	}

	changeRequest, err := http.NewRequest(http.MethodPost, "/api/v1/users/change-password", nil)
	if err != nil {
		t.Fatal(err)
	}
	changeRequest.Header.Set("Authorization", "Bearer "+token)
	changeResponse, err := app.Test(changeRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer changeResponse.Body.Close()
	if changeResponse.StatusCode != http.StatusOK {
		t.Fatalf("change-password status = %d, want %d", changeResponse.StatusCode, http.StatusOK)
	}
}
