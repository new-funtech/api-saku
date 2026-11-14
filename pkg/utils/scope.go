package utils

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Roles considered "Pusat" — they bypass tenant scoping entirely.
const (
	roleSuperAdmin = "super_admin"
	roleAdmin      = "admin"
	roleCompanyAdm = "company_admin"
	roleSupervisor = "supervisor"
	roleGuard      = "guard"
)

// IsPusat reports whether the role belongs to the central organisation.
func IsPusat(role string) bool {
	return role == roleSuperAdmin || role == roleAdmin
}

// IsBujpScoped reports whether the role is restricted to a single BUJP.
func IsBujpScoped(role string) bool {
	return role == roleCompanyAdm || role == roleSupervisor
}

// ApplyBujpScope clamps the filters map to the caller's BUJP / personnel
// when their role requires it. Pusat roles are unaffected.
//
// When a BUJP-scoped caller has no BUJP linked, filters["force_empty"]=true
// so repositories return an empty result instead of leaking other tenants' data.
func ApplyBujpScope(c *fiber.Ctx, filters map[string]interface{}) {
	if filters == nil {
		return
	}
	role, _ := c.Locals("role").(string)
	switch {
	case IsPusat(role):
		return
	case IsBujpScoped(role):
		bujpID, _ := c.Locals("bujpID").(uuid.UUID)
		if bujpID == uuid.Nil {
			filters["force_empty"] = true
			return
		}
		filters["bujp_id"] = bujpID
	case role == roleGuard:
		// Guards see only data tied to their own personnel record. We also
		// clamp bujp_id (when known) so list endpoints that expose master
		// data (locations, shifts, salary components, etc.) are limited to
		// the guard's BUJP scope as well.
		personnelID, _ := c.Locals("personnelID").(uuid.UUID)
		if personnelID == uuid.Nil {
			filters["force_empty"] = true
			return
		}
		filters["personnel_id"] = personnelID
		if bujpID, _ := c.Locals("bujpID").(uuid.UUID); bujpID != uuid.Nil {
			filters["bujp_id"] = bujpID
		}
	}
}

// CanAccessBujp returns true when the caller is allowed to read/write data
// belonging to the given BUJP.
func CanAccessBujp(c *fiber.Ctx, bujpID uuid.UUID) bool {
	role, _ := c.Locals("role").(string)
	if IsPusat(role) {
		return true
	}
	if IsBujpScoped(role) {
		caller, _ := c.Locals("bujpID").(uuid.UUID)
		return caller != uuid.Nil && caller == bujpID
	}
	return false // guards must use CanAccessPersonnel
}

// CanAccessOptionalBujp is like CanAccessBujp but for resources whose BujpID
// column is nullable (e.g. users, salary_components). Pusat is always allowed;
// BUJP-scoped callers require a non-nil ID that matches their tenant; resources
// with no tenant are treated as global and only Pusat may access them.
func CanAccessOptionalBujp(c *fiber.Ctx, bujpID *uuid.UUID) bool {
	role, _ := c.Locals("role").(string)
	if IsPusat(role) {
		return true
	}
	if bujpID == nil {
		return false
	}
	return CanAccessBujp(c, *bujpID)
}

// CanAccessPersonnel returns true when the caller is allowed to read/write
// data tied to the given personnel record.
func CanAccessPersonnel(c *fiber.Ctx, personnelID, personnelBujpID uuid.UUID) bool {
	role, _ := c.Locals("role").(string)
	if IsPusat(role) {
		return true
	}
	if IsBujpScoped(role) {
		caller, _ := c.Locals("bujpID").(uuid.UUID)
		return caller != uuid.Nil && caller == personnelBujpID
	}
	if role == roleGuard {
		caller, _ := c.Locals("personnelID").(uuid.UUID)
		return caller != uuid.Nil && caller == personnelID
	}
	return false
}

// CallerBujpID is the caller's company, taken from the server-side user record (never from the
// request). uuid.Nil for roles without a company (central roles).
func CallerBujpID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals("bujpID").(uuid.UUID)
	return id
}

// CallerRole is the role from the verified token.
func CallerRole(c *fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return role
}

// CanReadBujp is the read rule for company-owned records: central roles read everything, every
// other role (including a guard) reads only its own company.
func CanReadBujp(c *fiber.Ctx, bujpID uuid.UUID) bool {
	if IsPusat(CallerRole(c)) {
		return true
	}
	caller := CallerBujpID(c)
	return caller != uuid.Nil && caller == bujpID
}

// CanReadOptionalBujp is CanReadBujp for records whose company is optional; a record without a
// company is global and only central roles read it.
func CanReadOptionalBujp(c *fiber.Ctx, bujpID *uuid.UUID) bool {
	if IsPusat(CallerRole(c)) {
		return true
	}
	return bujpID != nil && CanReadBujp(c, *bujpID)
}
