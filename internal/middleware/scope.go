package middleware

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type callerScope struct {
	bujpID, personnelID uuid.UUID
	expires             time.Time
}

// scopeCache keeps who-belongs-to-which-company/personnel for a short time, so the two lookups
// every protected request used to make (users, personnels) run once per user per TTL instead.
// It is per process: with several API instances each keeps its own copy, and a change to a user's
// company or personnel link is seen after at most the TTL (SCOPE_CACHE_TTL_SECONDS, default 60;
// 0 turns the cache off).
type scopeCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	max     int
	entries map[uuid.UUID]callerScope
}

func newScopeCache() *scopeCache {
	ttl := 60
	if v, err := strconv.Atoi(os.Getenv("SCOPE_CACHE_TTL_SECONDS")); err == nil && v >= 0 {
		ttl = v
	}
	return &scopeCache{ttl: time.Duration(ttl) * time.Second, max: 50000, entries: map[uuid.UUID]callerScope{}}
}

func (s *scopeCache) get(id uuid.UUID) (callerScope, bool) {
	if s.ttl <= 0 {
		return callerScope{}, false
	}
	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()
	return e, ok && time.Now().Before(e.expires)
}

func (s *scopeCache) put(id uuid.UUID, e callerScope) {
	if s.ttl <= 0 {
		return
	}
	e.expires = time.Now().Add(s.ttl)
	s.mu.Lock()
	if len(s.entries) >= s.max {
		s.entries = map[uuid.UUID]callerScope{} // simple bound: start over rather than grow without limit
	}
	s.entries[id] = e
	s.mu.Unlock()
}

func LoadCallerScope(userRepo repository.UserRepository, personnelRepo repository.PersonnelRepository) fiber.Handler {
	cache := newScopeCache()
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("userID").(uuid.UUID)
		if uid == uuid.Nil {
			c.Locals("bujpID", uuid.Nil)
			c.Locals("personnelID", uuid.Nil)
			return c.Next()
		}

		scope, ok := cache.get(uid)
		if !ok {
			if user, err := userRepo.FindByID(c.Context(), uid); err == nil && user != nil && user.BujpID != nil {
				scope.bujpID = *user.BujpID
			}
			if p, err := personnelRepo.FindByUserID(c.Context(), uid); err == nil && p != nil {
				scope.personnelID = p.ID
			}
			cache.put(uid, scope)
		}

		c.Locals("bujpID", scope.bujpID)
		c.Locals("personnelID", scope.personnelID)
		return c.Next()
	}
}
