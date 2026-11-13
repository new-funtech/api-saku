package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type fakePrefRepo struct{ push, email bool }

func (f *fakePrefRepo) Get(context.Context, uuid.UUID) (bool, bool, error) {
	return f.push, f.email, nil
}
func (f *fakePrefRepo) Upsert(_ context.Context, _ uuid.UUID, push, email bool) error {
	f.push, f.email = push, email
	return nil
}
func (f *fakePrefRepo) EmailOptedOut(context.Context, []string) (map[string]bool, error) {
	return nil, nil
}

func prefApp(repo *fakePrefRepo) *fiber.App {
	app := fiber.New()
	h := NewNotificationPreferenceHandler(repo)
	user := uuid.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", user); return c.Next() })
	app.Get("/p", h.Get)
	app.Put("/p", h.Update)
	return app
}

func readPrefs(t *testing.T, app *fiber.App, method, body string) (int, map[string]bool) {
	req := httptest.NewRequest(method, "/p", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Data map[string]bool `json:"data"`
	}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out.Data
}

func TestPreferencesChangeOneSwitchAndKeepTheOther(t *testing.T) {
	repo := &fakePrefRepo{push: true, email: true}
	app := prefApp(repo)

	if code, d := readPrefs(t, app, "GET", ""); code != 200 || !d["push_enabled"] || !d["email_enabled"] {
		t.Fatalf("default: %d %v", code, d)
	}
	if code, d := readPrefs(t, app, "PUT", `{"email_enabled":false}`); code != 200 || !d["push_enabled"] || d["email_enabled"] {
		t.Fatalf("email off: %d %v", code, d)
	}
	if code, d := readPrefs(t, app, "PUT", `{"push_enabled":false}`); code != 200 || d["push_enabled"] || d["email_enabled"] {
		t.Fatalf("push off too: %d %v", code, d)
	}
	if code, _ := readPrefs(t, app, "PUT", `{}`); code != 400 {
		t.Fatalf("an empty update must be rejected, got %d", code)
	}
}
