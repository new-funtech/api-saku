package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type fakeDeviceRepo struct {
	upserts []string
	deleted []string
	owner   uuid.UUID
}

func (f *fakeDeviceRepo) Upsert(_ context.Context, userID uuid.UUID, token, platform string, _ *string) error {
	f.upserts = append(f.upserts, userID.String()+"|"+token+"|"+platform)
	return nil
}
func (f *fakeDeviceRepo) DeleteForUser(_ context.Context, userID uuid.UUID, token string) (bool, error) {
	f.deleted = append(f.deleted, userID.String()+"|"+token)
	return true, nil
}
func (f *fakeDeviceRepo) TokensByUser(context.Context, uuid.UUID) ([]string, error) { return nil, nil }
func (f *fakeDeviceRepo) PushEnabled(context.Context, uuid.UUID) bool               { return true }
func (f *fakeDeviceRepo) DeleteTokens(context.Context, []string) error              { return nil }

func deviceApp(repo *fakeDeviceRepo, user uuid.UUID) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		if errors.Is(err, middleware.ErrResponseWritten) {
			return nil
		}
		return fiber.DefaultErrorHandler(c, err)
	}})
	h := NewDeviceHandler(repo)
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", user); return c.Next() })
	app.Post("/devices/token", h.RegisterToken)
	app.Delete("/devices/token", h.UnregisterToken)
	return app
}

const sampleToken = "fcm-token-0123456789-abcdefghij"

func TestRegisterTokenStoresItForTheSignedInUser(t *testing.T) {
	repo := &fakeDeviceRepo{}
	user := uuid.New()
	app := deviceApp(repo, user)

	req := httptest.NewRequest("POST", "/devices/token", strings.NewReader(`{"fcm_token":"`+sampleToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("status %v err %v", resp, err)
	}
	if len(repo.upserts) != 1 || repo.upserts[0] != user.String()+"|"+sampleToken+"|android" {
		t.Fatalf("upserts = %v", repo.upserts)
	}
}

func TestRegisterTokenRejectsAMissingToken(t *testing.T) {
	app := deviceApp(&fakeDeviceRepo{}, uuid.New())
	req := httptest.NewRequest("POST", "/devices/token", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUnregisterTokenAcceptsBodyOrQuery(t *testing.T) {
	repo := &fakeDeviceRepo{}
	user := uuid.New()
	app := deviceApp(repo, user)

	req := httptest.NewRequest("DELETE", "/devices/token", strings.NewReader(`{"fcm_token":"`+sampleToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if resp, _ := app.Test(req); resp.StatusCode != 200 {
		t.Fatalf("body: status = %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest("DELETE", "/devices/token?fcm_token="+sampleToken, nil)); resp.StatusCode != 200 {
		t.Fatalf("query: status = %d", resp.StatusCode)
	}
	if len(repo.deleted) != 2 {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}
