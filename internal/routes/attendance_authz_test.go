package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/handlers"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// attendance service stub: one stored attendance, counts writes.
type stubAttendanceService struct {
	services.AttendanceService
	att     *model.AttendanceResponse
	updated int
	deleted int
}

func (s *stubAttendanceService) GetByID(_ context.Context, id uuid.UUID) (*model.AttendanceResponse, error) {
	if s.att == nil || s.att.ID != id {
		return nil, errors.New("not found")
	}
	cp := *s.att
	return &cp, nil
}
func (s *stubAttendanceService) Update(_ context.Context, id uuid.UUID, _ *model.UpdateAttendanceRequest) (*model.AttendanceResponse, error) {
	s.updated++
	return s.att, nil
}
func (s *stubAttendanceService) Delete(context.Context, uuid.UUID) error { s.deleted++; return nil }

type people struct {
	bujp      map[uuid.UUID]uuid.UUID // user -> company
	personnel map[uuid.UUID]uuid.UUID // user -> personnel
}

type fakeUsers struct {
	repository.UserRepository
	p people
}

func (f fakeUsers) FindByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	b := f.p.bujp[id]
	return &model.User{ID: id, BujpID: &b}, nil
}

type fakePersonnel struct {
	repository.PersonnelRepository
	p people
}

func (f fakePersonnel) FindByUserID(_ context.Context, id uuid.UUID) (*model.Personnel, error) {
	return &model.Personnel{ID: f.p.personnel[id]}, nil
}

type actor struct {
	token string
}

func newActor(t *testing.T, ppl people, role string, bujp, personnel uuid.UUID) actor {
	t.Helper()
	uid := uuid.New()
	ppl.bujp[uid], ppl.personnel[uid] = bujp, personnel
	tok, err := jwtutil.GenerateToken(uid, role+"@test.local", role)
	if err != nil {
		t.Fatal(err)
	}
	return actor{token: tok}
}

func TestAttendanceRowsAreProtectedByRoleAndCompany(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	companyA, companyB := uuid.New(), uuid.New()
	owner := uuid.New() // the employee who owns the attendance (company A)
	att := &model.AttendanceResponse{
		ID: uuid.New(), PersonnelID: owner,
		Personnel: &model.PersonnelResponse{ID: owner, BujpID: companyA},
	}
	svc := &stubAttendanceService{att: att}
	ppl := people{bujp: map[uuid.UUID]uuid.UUID{}, personnel: map[uuid.UUID]uuid.UUID{}}

	app := fiber.New()
	SetupRoutes(app, nil, nil, nil, nil, nil, nil, nil, nil, handlers.NewAttendanceHandler(svc), nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, fakeUsers{p: ppl}, fakePersonnel{p: ppl})

	call := func(a actor, method, id string) int {
		req := httptest.NewRequest(method, "/api/v1/attendances/"+id, strings.NewReader(`{"notes":"x"}`))
		req.Header.Set("Authorization", "Bearer "+a.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	id := att.ID.String()

	superAdmin := newActor(t, ppl, "super_admin", uuid.Nil, uuid.New())
	adminA := newActor(t, ppl, "company_admin", companyA, uuid.New())
	adminB := newActor(t, ppl, "company_admin", companyB, uuid.New())
	guardOwner := newActor(t, ppl, "guard", companyA, owner)
	guardOther := newActor(t, ppl, "guard", companyA, uuid.New())
	supervisorA := newActor(t, ppl, "supervisor", companyA, uuid.New())

	cases := []struct {
		name   string
		who    actor
		method string
		want   int
	}{
		// editing / deleting: admins only, and only inside their own company
		{"guard cannot edit even their own", guardOwner, "PUT", http.StatusForbidden},
		{"guard cannot delete even their own", guardOwner, "DELETE", http.StatusForbidden},
		{"supervisor cannot edit", supervisorA, "PUT", http.StatusForbidden},
		{"supervisor cannot delete", supervisorA, "DELETE", http.StatusForbidden},
		{"admin of another company cannot edit", adminB, "PUT", http.StatusForbidden},
		{"admin of another company cannot delete", adminB, "DELETE", http.StatusForbidden},
		{"admin of the same company can edit", adminA, "PUT", http.StatusOK},
		{"super admin can edit", superAdmin, "PUT", http.StatusOK},
		{"super admin can delete", superAdmin, "DELETE", http.StatusOK},
		// reading by id: the same row-level rule as the list
		{"owner can read", guardOwner, "GET", http.StatusOK},
		{"another guard cannot read", guardOther, "GET", http.StatusForbidden},
		{"admin of another company cannot read", adminB, "GET", http.StatusForbidden},
		{"admin of the same company can read", adminA, "GET", http.StatusOK},
	}
	for _, c := range cases {
		if got := call(c.who, c.method, id); got != c.want {
			t.Errorf("%s: %s -> %d, want %d", c.name, c.method, got, c.want)
		}
	}
	// What the denied calls must not have done: only the two allowed edits and one allowed delete went through.
	if svc.updated != 2 || svc.deleted != 1 {
		t.Errorf("writes: updated=%d deleted=%d, want 2 and 1 (a denied call must not reach the service)", svc.updated, svc.deleted)
	}
	if got := call(superAdmin, "PUT", uuid.NewString()); got != http.StatusNotFound {
		t.Errorf("unknown attendance -> %d, want 404", got)
	}
}
