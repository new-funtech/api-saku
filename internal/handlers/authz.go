package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Tenant isolation helpers shared by the handlers.
//
// The company (BUJP) of the caller always comes from the server-side user record loaded by
// middleware.LoadCallerScope, never from the body, query or path. A record of another company is
// answered with 403; an unknown id is 404.

// notFoundResponse is the uniform 404 for an id that does not exist.
func notFoundResponse(c *fiber.Ctx, what string) error {
	return c.Status(http.StatusNotFound).JSON(model.APIResponse{
		Status: "error", Code: http.StatusNotFound, Message: what + " not found",
	})
}

// ownCompany returns the company a write must be stored under. Central roles may name any company
// in the request; every other role is pinned to its own, and naming a different one is refused
// (ok=false). requested may be uuid.Nil (not named).
func ownCompany(c *fiber.Ctx, requested uuid.UUID) (company uuid.UUID, ok bool) {
	if utils.IsPusat(utils.CallerRole(c)) {
		return requested, true
	}
	own := utils.CallerBujpID(c)
	if own == uuid.Nil {
		return uuid.Nil, false
	}
	if requested != uuid.Nil && requested != own {
		return uuid.Nil, false
	}
	return own, true
}

// ownCompanyPtr is ownCompany for an optional company field.
func ownCompanyPtr(c *fiber.Ctx, requested *uuid.UUID) (*uuid.UUID, bool) {
	var want uuid.UUID
	if requested != nil {
		want = *requested
	}
	company, ok := ownCompany(c, want)
	if !ok {
		return nil, false
	}
	if company == uuid.Nil {
		return nil, true // a central role creating a global record
	}
	return &company, true
}

// --- ownership lookups -------------------------------------------------------------------------
//
// Records that belong to a person (leave, correction, patrol, assignment...) or to an office are
// checked against that person's / office's company. The repositories are set once at start-up.

var (
	personnelDirectory repository.PersonnelRepository
	locationDirectory  repository.LocationRepository
)

// UseDirectories wires the repositories the ownership checks read from.
func UseDirectories(p repository.PersonnelRepository, l repository.LocationRepository) {
	personnelDirectory, locationDirectory = p, l
}

// personAllowed reports whether the caller may act on a record that belongs to this person: central
// roles always, company roles inside their company, a guard only on their own record. owner may be
// the already-loaded person; otherwise it is looked up. Unknown people are refused.
func personAllowed(c *fiber.Ctx, ctx context.Context, personnelID uuid.UUID, owner *model.PersonnelResponse) bool {
	var bujp uuid.UUID
	switch {
	case owner != nil:
		bujp = owner.BujpID
	case personnelDirectory != nil && personnelID != uuid.Nil:
		p, err := personnelDirectory.FindByID(ctx, personnelID)
		if err != nil || p == nil {
			return false
		}
		bujp = p.BujpID
	default:
		return false
	}
	return utils.CanAccessPersonnel(c, personnelID, bujp)
}

// locationAllowed reports whether the caller may attach a record to this office.
func locationAllowed(c *fiber.Ctx, ctx context.Context, locationID uuid.UUID) bool {
	if locationDirectory == nil || locationID == uuid.Nil {
		return false
	}
	l, err := locationDirectory.FindByID(ctx, locationID)
	if err != nil || l == nil {
		return false
	}
	return utils.CanReadBujp(c, l.BujpID)
}

// assignableRoles lists the roles the caller may give to a user (create, or change to) and,
// equally, the roles of the users the caller may manage. This is what stops a company admin from
// minting platform-level accounts or editing another admin.
func assignableRoles(callerRole string) map[string]bool {
	switch callerRole {
	case "super_admin":
		return map[string]bool{"super_admin": true, "admin": true, "company_admin": true, "supervisor": true, "guard": true, "bprks": true}
	case "admin":
		return map[string]bool{"company_admin": true, "supervisor": true, "guard": true, "bprks": true}
	case "company_admin":
		return map[string]bool{"supervisor": true, "guard": true}
	}
	return nil
}

// --- stored files ------------------------------------------------------------------------------
//
// Object keys carry the id of the thing they belong to in their folder path (ATTENDANCES/<user>/...,
// PERSONNEL/KTP/<personnel>/..., LOANS/<user>/..., temp/<TYPE>/<entity>/...). A key is usable only if
// every such id is in the caller's scope. Keys without any id are the shared direct-upload folders
// (unguessable file names), which stay open to signed-in users.

var idInPath = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var sharedUploadFolders = map[string]bool{"temp": true, "users": true, "personnels": true, "products": true, "documents": true}

// cleanKey normalises an object key and refuses path tricks.
func cleanKey(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return "", false
	}
	return key, true
}

// tenantOfEntity finds the company of an entity id (a personnel, user, company, loan or office).
func tenantOfEntity(ctx context.Context, id uuid.UUID) (bujp uuid.UUID, found bool) {
	if config.DB == nil {
		return uuid.Nil, false
	}
	var rows []uuid.UUID
	err := config.DB.WithContext(ctx).Raw(`
		SELECT bujp_id FROM personnels WHERE id = ?
		UNION ALL SELECT bujp_id FROM users WHERE id = ? AND bujp_id IS NOT NULL
		UNION ALL SELECT id FROM bujps WHERE id = ?
		UNION ALL SELECT bujp_id FROM loans WHERE id = ? AND bujp_id IS NOT NULL
		UNION ALL SELECT bujp_id FROM locations WHERE id = ?`, id, id, id, id, id).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return uuid.Nil, false
	}
	return rows[0], true
}

// entityAllowed: the caller's own ids always; otherwise the entity must belong to the caller's
// company (a guard is limited to their own ids).
func entityAllowed(c *fiber.Ctx, ctx context.Context, id uuid.UUID) bool {
	if id == utils.CallerBujpID(c) && utils.CallerRole(c) != "guard" {
		return true
	}
	if uid, _ := c.Locals("userID").(uuid.UUID); id == uid {
		return true
	}
	if pid, _ := c.Locals("personnelID").(uuid.UUID); pid != uuid.Nil && id == pid {
		return true
	}
	if utils.CallerRole(c) == "guard" {
		return false
	}
	bujp, found := tenantOfEntity(ctx, id)
	own := utils.CallerBujpID(c)
	return found && own != uuid.Nil && bujp == own
}

// entityAllowedOrNew is entityAllowed, plus an id that belongs to nothing yet: an admin adding a
// record picks its id before saving so the first upload has a folder. An id that exists in another
// company is still refused.
func entityAllowedOrNew(c *fiber.Ctx, ctx context.Context, id uuid.UUID) bool {
	if entityAllowed(c, ctx, id) {
		return true
	}
	if utils.CallerRole(c) == "guard" {
		return false
	}
	_, found := tenantOfEntity(ctx, id)
	return !found
}

// fileKeyAllowed reports whether the caller may sign, move or delete this object key.
func fileKeyAllowed(c *fiber.Ctx, ctx context.Context, key string) bool {
	if utils.IsPusat(utils.CallerRole(c)) {
		return true
	}
	key, ok := cleanKey(key)
	if !ok {
		return false
	}
	segments := strings.Split(key, "/")
	dirs := segments[:len(segments)-1] // the last part is the file name
	var ids []uuid.UUID
	for _, s := range dirs {
		if idInPath.MatchString(s) {
			id, _ := uuid.Parse(s)
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return sharedUploadFolders[strings.ToLower(segments[0])]
	}
	for _, id := range ids {
		if !entityAllowed(c, ctx, id) {
			return false
		}
	}
	return true
}

// moveOwnedFile moves an object only when both keys are in the caller's scope.
func moveOwnedFile(c *fiber.Ctx, ctx context.Context, src, dst string) error {
	if !fileKeyAllowed(c, ctx, src) || !fileKeyAllowed(c, ctx, dst) {
		return errors.New("file path is outside your scope")
	}
	return utils.MoveFileInS3(ctx, src, dst)
}
