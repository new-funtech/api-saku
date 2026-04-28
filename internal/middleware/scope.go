package middleware

import (
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// LoadCallerScope resolves the authenticated user's BUJP and Personnel IDs
// once per request and stores them in c.Locals so downstream handlers can
// enforce tenant scoping without repeating DB lookups.
//
// Locals set:
//   - "bujpID"      uuid.UUID (uuid.Nil if user is unbound)
//   - "personnelID" uuid.UUID (uuid.Nil if user has no personnel record)
//
// Must run AFTER AuthRequired.
func LoadCallerScope(userRepo repository.UserRepository, personnelRepo repository.PersonnelRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("userID").(uuid.UUID)
		if uid == uuid.Nil {
			c.Locals("bujpID", uuid.Nil)
			c.Locals("personnelID", uuid.Nil)
			return c.Next()
		}

		var bujpID, personnelID uuid.UUID
		if user, err := userRepo.FindByID(c.Context(), uid); err == nil && user != nil && user.BujpID != nil {
			bujpID = *user.BujpID
		}
		if p, err := personnelRepo.FindByUserID(c.Context(), uid); err == nil && p != nil {
			personnelID = p.ID
		}

		c.Locals("bujpID", bujpID)
		c.Locals("personnelID", personnelID)
		return c.Next()
	}
}
