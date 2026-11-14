package tenantiso

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (w *world) rawGet(a actor, path string) (*http.Response, []byte) {
	w.t.Helper()
	req, _ := http.NewRequest("GET", w.base+"/api/v1"+path, nil)
	req.Header.Set("Authorization", "Bearer "+a.token)
	resp, err := w.client.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func thisMonth() string { return time.Now().Format("2006-01") }

func TestMonthlyRecapIsScopedAndCorrect(t *testing.T) {
	w := setup(t)
	a, b := w.A, w.B
	period := thisMonth()

	// the company admin sees their own people, with the right numbers, and nothing of the other company
	r := w.do(a.admin, "GET", "/monthly-reports/recap?period="+period, nil)
	if !r.ok() {
		t.Fatalf("recap: %d %s", r.status, trim(r.body))
	}
	if hit := r.leaks(b.ids); len(hit) > 0 {
		t.Errorf("recap leaks the other company: %v", hit)
	}
	var out struct {
		Data struct {
			Rows []struct {
				FullName string `json:"full_name"`
				Hadir    int    `json:"hadir"`
			} `json:"rows"`
			Totals struct {
				Hadir int `json:"hadir"`
			} `json:"totals"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(r.body), &out)
	if len(out.Data.Rows) < 2 {
		t.Errorf("rows = %d, want every person of the company (3 seeded)", len(out.Data.Rows))
	}
	if out.Data.Totals.Hadir != 1 {
		t.Errorf("total hadir = %d, want 1 (the guard has one present attendance today)", out.Data.Totals.Hadir)
	}

	// filters: person, office and name search
	if r := w.do(a.admin, "GET", "/monthly-reports/recap?period="+period+"&personnel_id="+a.personnel.ID.String(), nil); !r.ok() || strings.Contains(r.body, a.extraPersonnel.FullName) {
		t.Errorf("personnel filter: %d", r.status)
	}
	if r := w.do(a.admin, "GET", "/monthly-reports/recap?period="+period+"&search=Guard", nil); !r.ok() || !strings.Contains(r.body, "Guard A") || strings.Contains(r.body, "Extra A") {
		t.Errorf("search filter: %d %s", r.status, trim(r.body))
	}
	if r := w.do(a.admin, "GET", "/monthly-reports/recap?period="+period+"&location_id="+a.location.ID.String(), nil); !r.ok() || !strings.Contains(r.body, "Guard A") {
		t.Errorf("office filter: %d %s", r.status, trim(r.body))
	}

	// refusals
	for name, path := range map[string]string{
		"another company's office":  "/monthly-reports/recap?period=" + period + "&location_id=" + b.location.ID.String(),
		"another company's person":  "/monthly-reports/recap?period=" + period + "&personnel_id=" + b.personnel.ID.String(),
		"another company's bujp_id": "/monthly-reports/recap?period=" + period + "&bujp_id=" + b.bujp.ID.String(),
	} {
		got := w.do(a.admin, "GET", path, nil)
		if got.status == 403 || got.status == 404 {
			continue
		}
		// naming another company in bujp_id is ignored for non-central roles: the answer must still be company A's
		if name == "another company's bujp_id" && got.ok() && len(got.leaks(b.ids)) == 0 {
			continue
		}
		t.Errorf("%s: status %d, want a refusal (or a recap of the caller's own company)", name, got.status)
	}
	for _, who := range []actor{a.supervisor, a.guard} {
		if r := w.do(who, "GET", "/monthly-reports/recap?period="+period, nil); r.ok() {
			t.Errorf("%s must not read the monthly recap", who.user.Role)
		}
		if resp, _ := w.rawGet(who, "/monthly-reports/recap/export?period="+period); resp.StatusCode < 400 {
			t.Errorf("%s must not export the monthly recap", who.user.Role)
		}
	}
	if r := w.do(a.admin, "GET", "/monthly-reports/recap?period=2024-13", nil); r.status != 400 {
		t.Errorf("invalid period: status %d, want 400", r.status)
	}
	if r := w.do(w.super, "GET", "/monthly-reports/recap?period="+period, nil); r.status != 400 {
		t.Errorf("a central role must name a company: status %d, want 400", r.status)
	}
	if r := w.do(w.super, "GET", "/monthly-reports/recap?period="+period+"&bujp_id="+b.bujp.ID.String(), nil); !r.ok() || !strings.Contains(r.body, "Guard B") {
		t.Errorf("a central role can read any company: %d", r.status)
	}

	// the Excel export is the same data
	resp, body := w.rawGet(a.admin, "/monthly-reports/recap/export?period="+period)
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "spreadsheetml") || len(body) < 1000 || string(body[:2]) != "PK" {
		t.Errorf("export: status %d type %q size %d", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
	if strings.Contains(string(body), "Guard B") {
		t.Error("the export contains the other company")
	}
}

func TestAttendanceHistoryOfEveryPerson(t *testing.T) {
	w := setup(t)
	a, b := w.A, w.B
	today := time.Now().Format("2006-01-02")

	r := w.do(a.admin, "GET", "/attendances?per_page=100&date_from="+today+"&date_to="+today, nil)
	if !r.ok() {
		t.Fatalf("history: %d", r.status)
	}
	for _, want := range []string{"Guard A", a.personnel.IDNumber, "Location A", "Shift A", "present"} {
		if !strings.Contains(r.body, want) {
			t.Errorf("the history row lacks %q: %s", want, trim(r.body))
		}
	}
	if hit := r.leaks(b.ids); len(hit) > 0 {
		t.Errorf("history leaks the other company: %v", hit)
	}
	if r := w.do(a.admin, "GET", "/attendances?search=Guard", nil); !strings.Contains(r.body, "Guard A") {
		t.Errorf("name search finds nothing")
	}
	if r := w.do(a.admin, "GET", "/attendances?search="+b.personnel.FullName, nil); strings.Contains(r.body, b.personnel.IDNumber) {
		t.Errorf("searching the other company's name returned its rows")
	}
	if r := w.do(a.admin, "GET", "/attendances?date_from=2001-01-01&date_to=2001-01-02", nil); strings.Contains(r.body, "Guard A") {
		t.Errorf("the date range is ignored")
	}
	if r := w.do(a.admin, "GET", "/attendances?status=absent", nil); strings.Contains(r.body, "Guard A") {
		t.Errorf("the status filter is ignored")
	}
	if r := w.do(a.admin, "GET", "/attendances?location_id="+b.location.ID.String(), nil); strings.Contains(r.body, b.personnel.IDNumber) {
		t.Errorf("another company's office filter returned rows")
	}
}

func TestACompanyAdminCanSwitchAnAccountOff(t *testing.T) {
	w := setup(t)
	a := w.A
	if r := w.do(a.admin, "PUT", "/users/"+a.guard.user.ID.String(), map[string]interface{}{"status": "inactive"}); !r.ok() {
		t.Fatalf("deactivate: %d %s", r.status, trim(r.body))
	}
	if r := w.do(a.guard, "GET", "/locations", nil); r.status != 401 {
		t.Errorf("the switched-off account was still served (status %d)", r.status)
	}
	if r := w.do(a.admin, "PUT", "/users/"+a.guard.user.ID.String(), map[string]interface{}{"status": "active"}); !r.ok() {
		t.Fatalf("reactivate: %d", r.status)
	}
	if r := w.do(a.guard, "GET", "/locations", nil); !r.ok() {
		t.Errorf("a reactivated account must work again (status %d)", r.status)
	}
	_ = uuid.Nil
}

// TestCompanyAdminCanRunTheirMasterData proves the lock-down did not over-reach: everything a
// company admin needs to manage in their own company still works (create, change, switch off).
func TestCompanyAdminCanRunTheirMasterData(t *testing.T) {
	w := setup(t)
	a := w.A
	admin := a.admin
	id := func(r reply) string {
		var out struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal([]byte(r.body), &out)
		return out.Data.ID
	}
	must := func(what string, r reply) reply {
		t.Helper()
		if !r.ok() {
			t.Fatalf("%s: %d %s", what, r.status, trim(r.body))
		}
		return r
	}
	u := w.suffix + uuid.NewString()[:4]

	// location
	loc := must("create location", w.do(admin, "POST", "/locations", map[string]interface{}{"bujp_id": a.bujp.ID.String(), "code": "N" + u, "name": "New " + u, "address": "Jl Baru", "latitude": -6.2, "longitude": 106.8, "attendance_radius": 100}))
	locID := id(loc)
	must("update location", w.do(admin, "PUT", "/locations/"+locID, map[string]interface{}{"name": "Renamed " + u}))
	// shift
	sh := must("create shift", w.do(admin, "POST", "/shifts", map[string]interface{}{"name": "Pagi " + u, "start_time": "07:00", "end_time": "15:00"}))
	shID := id(sh)
	must("update shift", w.do(admin, "PUT", "/shifts/"+shID, map[string]interface{}{"name": "Pagi2 " + u}))
	// salary component
	sc := must("create salary component", w.do(admin, "POST", "/salary-components", map[string]interface{}{"code": "T" + u[:8], "name": "Tunjangan " + u, "type": "allowance", "calculation_method": "fixed", "amount": 50000, "is_active": true}))
	scID := id(sc)
	must("update salary component", w.do(admin, "PUT", "/salary-components/"+scID, map[string]interface{}{"amount": 60000}))
	// personnel: create, change, switch off
	pe := must("create personnel", w.do(admin, "POST", "/personnels", map[string]interface{}{"full_name": "Orang Baru " + u, "id_number": "3" + strings.Repeat("1", 15), "nip": "N" + u, "join_date": "2024-01-01", "status": "active"}))
	peID := id(pe)
	must("update personnel", w.do(admin, "PUT", "/personnels/"+peID, map[string]interface{}{"full_name": "Orang Baru Ubah " + u}))
	must("switch personnel off", w.do(admin, "PUT", "/personnels/"+peID, map[string]interface{}{"status": "inactive"}))
	// user: create a guard, change, switch off
	us := must("create user", w.do(admin, "POST", "/users", map[string]interface{}{"email": "g-" + u + "@iso.test", "password": "Passw0rd!x", "role": "guard", "full_name": "Guard Baru"}))
	usID := id(us)
	must("update user", w.do(admin, "PUT", "/users/"+usID, map[string]interface{}{"full_name": "Guard Baru 2"}))
	must("switch user off", w.do(admin, "PUT", "/users/"+usID, map[string]interface{}{"status": "inactive"}))
	// assignment
	must("create assignment", w.do(admin, "POST", "/assignments", map[string]interface{}{"personnel_id": a.extraPersonnel.ID.String(), "location_id": locID, "shift_id": shID, "start_date": "2024-01-01"}))
	// approvals of their own company's requests
	must("approve leave", w.do(admin, "POST", "/leaves/"+a.leave.ID.String()+"/approve", map[string]interface{}{"status": "approved", "approver_notes": "ok"}))
	must("approve correction", w.do(admin, "POST", "/attendance-corrections/"+a.correction.ID.String()+"/approve", map[string]interface{}{"status": "approved", "approver_notes": "ok"}))
	must("approve overtime", w.do(admin, "POST", "/overtimes/"+a.overtime.ID.String()+"/approve", map[string]interface{}{"approver_notes": "ok"}))
	// payroll and the monthly report for their own company
	must("generate payroll", w.do(admin, "POST", "/payrolls/generate", map[string]interface{}{"bujp_id": a.bujp.ID.String(), "period": "2021-06"}))
	must("generate report", w.do(admin, "POST", "/monthly-reports/generate", map[string]interface{}{"period": "2021-06"}))
	must("delete salary component", w.do(admin, "DELETE", "/salary-components/"+scID, nil))
	must("delete shift", w.do(admin, "DELETE", "/shifts/"+shID, nil))
}

func TestListSearchesStayInsideTheCompany(t *testing.T) {
	w := setup(t)
	a, b := w.A, w.B
	for path, want := range map[string]string{
		"/salary-components?search=Component": "Component A",
		"/locations?search=Location":          "Location A",
		"/shifts?search=Shift":                "Shift A",
		"/personnels?search=Guard":            "Guard A",
		"/users?search=guard":                 "guardA",
	} {
		r := w.do(a.admin, "GET", path, nil)
		if !r.ok() || !strings.Contains(r.body, want) {
			t.Errorf("GET %s should find %q: %d %s", path, want, r.status, trim(r.body))
		}
		if hit := r.leaks(b.ids); len(hit) > 0 {
			t.Errorf("GET %s leaks the other company: %v", path, hit)
		}
	}
}
