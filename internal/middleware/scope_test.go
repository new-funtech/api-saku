package middleware

import (
	"context"
	"gorm.io/gorm"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type countingUsers struct {
	repository.UserRepository
	calls int
	bujp  uuid.UUID
}

func (c *countingUsers) FindByID(context.Context, uuid.UUID) (*model.User, error) {
	c.calls++
	return &model.User{BujpID: &c.bujp}, nil
}

type countingPersonnel struct {
	repository.PersonnelRepository
	calls int
	id    uuid.UUID
}

func (c *countingPersonnel) FindByUserID(context.Context, uuid.UUID) (*model.Personnel, error) {
	c.calls++
	return &model.Personnel{ID: c.id}, nil
}

func scopeApp(u *countingUsers, p *countingPersonnel, user uuid.UUID) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", user); return c.Next() })
	app.Use(LoadCallerScope(u, p))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Locals("personnelID").(uuid.UUID).String())
	})
	return app
}

func TestScopeIsLookedUpOncePerTTL(t *testing.T) {
	t.Setenv("SCOPE_CACHE_TTL_SECONDS", "60")
	u, p := &countingUsers{bujp: uuid.New()}, &countingPersonnel{id: uuid.New()}
	app := scopeApp(u, p, uuid.New())
	for i := 0; i < 5; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("request %d: %v %v", i, resp, err)
		}
	}
	if u.calls != 1 || p.calls != 1 {
		t.Errorf("lookups = %d users + %d personnel over 5 requests, want 1 + 1", u.calls, p.calls)
	}
}

func TestScopeCacheCanBeDisabled(t *testing.T) {
	t.Setenv("SCOPE_CACHE_TTL_SECONDS", "0")
	u, p := &countingUsers{bujp: uuid.New()}, &countingPersonnel{id: uuid.New()}
	app := scopeApp(u, p, uuid.New())
	for i := 0; i < 3; i++ {
		if _, err := app.Test(httptest.NewRequest("GET", "/", nil)); err != nil {
			t.Fatal(err)
		}
	}
	if u.calls != 3 {
		t.Errorf("user lookups = %d, want 3 with the cache off", u.calls)
	}
}

func TestScopeCacheExpires(t *testing.T) {
	c := &scopeCache{ttl: 10 * time.Millisecond, max: 10, entries: map[uuid.UUID]callerScope{}}
	id := uuid.New()
	c.put(id, callerScope{personnelID: uuid.New()})
	if _, ok := c.get(id); !ok {
		t.Fatal("fresh entry must hit")
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.get(id); ok {
		t.Fatal("expired entry must miss")
	}
}

type statusUsers struct {
	repository.UserRepository
	user *model.User
	err  error
}

func (s statusUsers) FindByID(context.Context, uuid.UUID) (*model.User, error) { return s.user, s.err }

func roleApp(users repository.UserRepository, tokenRole string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", uuid.New())
		c.Locals("role", tokenRole)
		return c.Next()
	})
	app.Use(LoadCallerScope(users, &countingPersonnel{id: uuid.New()}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(c.Locals("role").(string)) })
	return app
}

func TestDeactivatedAccountIsRefusedEvenWithAValidToken(t *testing.T) {
	t.Setenv("SCOPE_CACHE_TTL_SECONDS", "0")
	app := roleApp(statusUsers{user: &model.User{Role: "guard", Status: "inactive"}}, "guard")
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || resp.StatusCode != 401 {
		t.Fatalf("status = %v (%v), want 401 for an inactive account", resp, err)
	}
}

func TestADeletedAccountIsRefused(t *testing.T) {
	t.Setenv("SCOPE_CACHE_TTL_SECONDS", "0")
	app := roleApp(statusUsers{err: gorm.ErrRecordNotFound}, "super_admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || resp.StatusCode != 401 {
		t.Fatalf("status = %v (%v), want 401 for a removed account", resp, err)
	}
}

func TestTheStoredRoleOverridesAnOldToken(t *testing.T) {
	t.Setenv("SCOPE_CACHE_TTL_SECONDS", "0")
	// The token still says company_admin, but the account was demoted to guard.
	app := roleApp(statusUsers{user: &model.User{Role: "guard", Status: "active"}}, "company_admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "guard" {
		t.Fatalf("role seen by the handlers = %q, want the stored role guard", body)
	}
}
