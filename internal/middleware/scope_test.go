package middleware

import (
	"context"
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
