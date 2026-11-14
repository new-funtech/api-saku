package tenantiso

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// attack is one request an outsider must not be able to make succeed.
type attack struct {
	name   string
	method string
	path   string
	body   interface{}
	table  string    // the victim row to check afterwards ("" = none)
	id     uuid.UUID // victim row id
}

func (w *world) victimAttacks(v *tenant) []attack {
	p := func(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
	var out []attack
	add := func(name, method, path string, body interface{}, table string, id uuid.UUID) {
		out = append(out, attack{name, method, path, body, table, id})
	}
	// by-id read / change / delete of every tenant-owned resource
	type res struct {
		path, table string
		id          uuid.UUID
		put         interface{}
	}
	for _, r := range []res{
		{"/bujps", "bujps", v.bujp.ID, map[string]interface{}{"name": "hijacked"}},
		{"/locations", "locations", v.location.ID, map[string]interface{}{"name": "hijacked"}},
		{"/shifts", "shifts", v.shift.ID, map[string]interface{}{"name": "hijacked"}},
		{"/personnels", "personnels", v.personnel.ID, map[string]interface{}{"full_name": "hijacked"}},
		{"/users", "users", v.guard.user.ID, map[string]interface{}{"full_name": "hijacked"}},
		{"/users", "users", v.admin.user.ID, map[string]interface{}{"full_name": "hijacked", "role": "guard"}},
		{"/salary-components", "salary_components", v.component.ID, map[string]interface{}{"name": "hijacked"}},
		{"/assignments", "assignments", v.assignment.ID, map[string]interface{}{"notes": "hijacked"}},
		{"/attendances", "attendances", v.attendance.ID, map[string]interface{}{"notes": "hijacked"}},
		{"/attendance-corrections", "attendance_corrections", v.correction.ID, map[string]interface{}{"reason": "hijacked"}},
		{"/leaves", "leaves", v.leave.ID, map[string]interface{}{"reason": "hijacked"}},
		{"/overtimes", "overtimes", v.overtime.ID, map[string]interface{}{"reason": "hijacked"}},
		{"/patrols", "patrols", v.patrol.ID, map[string]interface{}{"notes": "hijacked"}},
		{"/payrolls", "payrolls", v.payroll.ID, map[string]interface{}{"base_salary": 1}},
		{"/monthly-reports", "monthly_reports", v.report.ID, map[string]interface{}{"total_present": 99}},
		{"/loan-products", "loan_products", v.product.ID, map[string]interface{}{"name": "hijacked"}},
		{"/loans", "loans", v.loan.ID, map[string]interface{}{"purpose": "hijacked"}},
	} {
		name := strings.TrimPrefix(r.path, "/")
		add("GET "+name, "GET", p("%s/%s", r.path, r.id), nil, r.table, r.id)
		add("PUT "+name, "PUT", p("%s/%s", r.path, r.id), r.put, r.table, r.id)
		add("DELETE "+name, "DELETE", p("%s/%s", r.path, r.id), nil, r.table, r.id)
	}
	// decisions and state changes on someone else's requests
	for _, d := range []struct {
		name, path string
		body       interface{}
		table      string
		id         uuid.UUID
	}{
		{"approve leave", p("/leaves/%s/approve", v.leave.ID), map[string]interface{}{"status": "approved", "approver_notes": "x"}, "leaves", v.leave.ID},
		{"reject leave", p("/leaves/%s/reject", v.leave.ID), map[string]interface{}{"approver_notes": "x"}, "leaves", v.leave.ID},
		{"approve correction", p("/attendance-corrections/%s/approve", v.correction.ID), map[string]interface{}{"status": "approved", "approver_notes": "x"}, "attendance_corrections", v.correction.ID},
		{"reject correction", p("/attendance-corrections/%s/reject", v.correction.ID), map[string]interface{}{"approver_notes": "x"}, "attendance_corrections", v.correction.ID},
		{"approve overtime", p("/overtimes/%s/approve", v.overtime.ID), map[string]interface{}{"approver_notes": "x"}, "overtimes", v.overtime.ID},
		{"reject overtime", p("/overtimes/%s/reject", v.overtime.ID), map[string]interface{}{"approver_notes": "x"}, "overtimes", v.overtime.ID},
		{"validate patrol", p("/patrols/%s/validate", v.patrol.ID), map[string]interface{}{"validation_status": "valid"}, "patrols", v.patrol.ID},
		{"payroll payment status", p("/payrolls/%s/update-payment-status", v.payroll.ID), map[string]interface{}{"payment_status": "paid"}, "payrolls", v.payroll.ID},
		{"approve loan", p("/loan-approvals/%s/approve", v.loan.ID), map[string]interface{}{"notes": "x"}, "loans", v.loan.ID},
		{"reject loan", p("/loan-approvals/%s/reject", v.loan.ID), map[string]interface{}{"notes": "x"}, "loans", v.loan.ID},
		{"process loan approval", p("/loan-approvals/%s/process", v.approval.ID), map[string]interface{}{"action": "approve"}, "loan_approvals", v.approval.ID},
		{"read loan approvals", p("/loan-approvals/loan/%s", v.loan.ID), nil, "loans", v.loan.ID},
		{"disburse loan", p("/loan-approvals/loan/%s/disburse", v.loan.ID), map[string]interface{}{}, "loans", v.loan.ID},
		{"cancel loan", p("/loans/%s/cancel", v.loan.ID), map[string]interface{}{}, "loans", v.loan.ID},
		{"submit loan", p("/loans/%s/submit", v.loan.ID), map[string]interface{}{}, "loans", v.loan.ID},
		{"read loan installments", p("/loans/%s/installments", v.loan.ID), nil, "loans", v.loan.ID},
		{"loan history of their personnel", p("/loans/history/%s", v.personnel.ID), nil, "", uuid.Nil},
		{"read their notification (delete)", p("/notifications/%s", v.notification.ID), nil, "notifications", v.notification.ID},
		{"mark their notification read", p("/notifications/%s/read", v.notification.ID), map[string]interface{}{}, "notifications", v.notification.ID},
	} {
		method := "POST"
		if d.body == nil {
			method = "GET"
		}
		if strings.HasPrefix(d.name, "read their") {
			method = "DELETE"
		}
		add(d.name, method, d.path, d.body, d.table, d.id)
	}
	return out
}

// creates that would put data into the victim's tenant, or give a caller more power than they have.
func (w *world) creations(a *tenant, v *tenant) []attack {
	vb := v.bujp.ID.String()
	uniq := func(s string) string { return s + w.suffix + uuid.NewString()[:4] }
	return []attack{
		{"create location in the other company", "POST", "/locations", map[string]interface{}{"bujp_id": vb, "code": uniq("X"), "name": "planted", "address": "x", "latitude": -6.2, "longitude": 106.8}, "", uuid.Nil},
		{"create shift in the other company", "POST", "/shifts", map[string]interface{}{"bujp_id": vb, "name": uniq("planted"), "start_time": "08:00", "end_time": "16:00"}, "", uuid.Nil},
		{"create personnel in the other company", "POST", "/personnels", map[string]interface{}{"bujp_id": vb, "id_number": fmt.Sprintf("%016d", time.Now().UnixNano()%10000000000000000), "full_name": "planted", "join_date": "2024-01-01"}, "", uuid.Nil},
		{"create user in the other company", "POST", "/users", map[string]interface{}{"bujp_id": vb, "email": uniq("p") + "@iso.test", "password": "Passw0rd!x", "role": "guard", "full_name": "planted"}, "", uuid.Nil},
		{"create user with a platform role", "POST", "/users", map[string]interface{}{"email": uniq("s") + "@iso.test", "password": "Passw0rd!x", "role": "super_admin", "full_name": "escalated"}, "", uuid.Nil},
		{"create a platform admin in own company", "POST", "/users", map[string]interface{}{"bujp_id": a.bujp.ID.String(), "email": uniq("a") + "@iso.test", "password": "Passw0rd!x", "role": "admin", "full_name": "escalated"}, "", uuid.Nil},
		{"create salary component in the other company", "POST", "/salary-components", map[string]interface{}{"bujp_id": vb, "code": uniq("S")[:10], "name": "planted", "type": "allowance", "calculation_method": "fixed", "amount": 1}, "", uuid.Nil},
		{"create loan product in the other company", "POST", "/loan-products", map[string]interface{}{"bujp_id": vb, "code": uniq("P")[:10], "name": "planted"}, "", uuid.Nil},
		{"create a company", "POST", "/bujps", map[string]interface{}{"code": uniq("B")[:10], "name": "planted company"}, "", uuid.Nil},
		{"assign the other company's personnel", "POST", "/assignments", map[string]interface{}{"personnel_id": v.personnel.ID.String(), "location_id": a.location.ID.String(), "start_date": "2024-01-01"}, "", uuid.Nil},
		{"assign own personnel to the other company's location", "POST", "/assignments", map[string]interface{}{"personnel_id": a.personnel.ID.String(), "location_id": v.location.ID.String(), "start_date": "2024-01-01"}, "", uuid.Nil},
		{"generate payroll for the other company", "POST", "/payrolls/generate", map[string]interface{}{"bujp_id": vb, "period": "2021-02"}, "", uuid.Nil},
		{"generate report for the other company", "POST", "/monthly-reports/generate", map[string]interface{}{"bujp_id": vb, "period": "2021-02"}, "", uuid.Nil},
		{"create payroll for the other company's personnel", "POST", "/payrolls", map[string]interface{}{"personnel_id": v.personnel.ID.String(), "bujp_id": vb, "period": "2021-03", "base_salary": 1}, "", uuid.Nil},
		{"create leave for the other company's personnel", "POST", "/leaves", map[string]interface{}{"personnel_id": v.personnel.ID.String(), "type": "leave", "start_date": "2030-01-01", "end_date": "2030-01-02", "reason": "planted"}, "", uuid.Nil},
	}
}

// Requests that name another company are answered either with a refusal or by silently storing the
// record in the caller's own company (the company always comes from the session). The second is
// fine for a company admin as long as nothing lands in the other company (checked separately).
var pinnedToOwnCompany = map[string]bool{
	"create location in the other company": true, "create shift in the other company": true,
	"create personnel in the other company": true, "create user in the other company": true,
	"create salary component in the other company": true, "create loan product in the other company": true,
	"generate payroll for the other company": true, "generate report for the other company": true,
}

// Deleting or marking read someone else's notification does nothing to it (the lookup is by
// recipient); a 200 there is harmless as long as the row is untouched.
var harmlessNoop = map[string]bool{"read their notification (delete)": true, "mark their notification read": true}

func denied(r reply) bool {
	return r.status == 401 || r.status == 403 || r.status == 404 || r.status == 400 || r.status == 422
}

// TestOutsidersCannotTouchAnotherCompany: company A's admin, supervisor and guard against company B.
// Every attacker gets a freshly seeded pair of companies, so an earlier (successful) attack cannot
// hide a later one.
func TestOutsidersCannotTouchAnotherCompany(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, role := range []string{"company_admin", "supervisor", "guard"} {
			w := setup(t)
			attacker, victim := w.A, w.B
			if reverse {
				attacker, victim = w.B, w.A
			}
			who := map[string]actor{"company_admin": attacker.admin, "supervisor": attacker.supervisor, "guard": attacker.guard}[role]
			for _, a := range w.victimAttacks(victim) {
				before := ""
				if a.table != "" {
					before = w.snapshot(a.table, a.id)
				}
				r := w.do(who, a.method, a.path, a.body)
				label := fmt.Sprintf("%s [%s of %s] %s %s", a.name, role, attacker.tag, a.method, a.path)
				unchanged := a.table != "" && w.snapshot(a.table, a.id) == before
				if r.ok() && !(harmlessNoop[a.name] && unchanged) {
					t.Errorf("ALLOWED       %s: status %d", label, r.status)
				} else if r.status >= 500 {
					t.Errorf("SERVER ERROR  %s: status %d (%s)", label, r.status, trim(r.body))
				}
				if hit := r.leaks(victim.ids); len(hit) > 0 && !r.ok() {
					t.Errorf("BODY LEAKS    %s: the error response contains %v", label, hit)
				}
				if a.table != "" {
					if after := w.snapshot(a.table, a.id); after != before {
						t.Errorf("DATA CHANGED  %s: %s row was modified or deleted", label, a.table)
					}
				}
			}
		}
	}
}

func TestOutsidersCannotCreateIntoAnotherCompanyOrEscalate(t *testing.T) {
	w := setup(t)
	for _, who := range []func(*tenant) actor{func(x *tenant) actor { return x.admin }, func(x *tenant) actor { return x.supervisor }, func(x *tenant) actor { return x.guard }} {
		for _, a := range w.creations(w.A, w.B) {
			actorA := who(w.A)
			r := w.do(actorA, a.method, a.path, a.body)
			pinnedOK := pinnedToOwnCompany[a.name] && actorA.user.Role == "company_admin"
			if r.ok() && !pinnedOK {
				t.Errorf("ALLOWED  %s by %s of A: status %d (%s)", a.name, actorA.user.Role, r.status, trim(r.body))
			} else if r.status >= 500 {
				t.Errorf("SERVER ERROR  %s by %s: %d (%s)", a.name, actorA.user.Role, r.status, trim(r.body))
			}
		}
	}
	// nothing was planted in company B
	for table, col := range map[string]string{"locations": "bujp_id", "personnels": "bujp_id", "users": "bujp_id", "salary_components": "bujp_id", "loan_products": "bujp_id", "payrolls": "bujp_id", "monthly_reports": "bujp_id"} {
		var n int64
		w.db.Table(table).Where(col+" = ? AND created_at > now() - interval '5 minutes'", w.B.bujp.ID).Where("id NOT IN (?)", w.knownIDs(w.B)).Count(&n)
		if n > 0 {
			t.Errorf("PLANTED  %d new row(s) in %s for company B", n, table)
		}
	}
	for _, table := range []string{"payrolls", "monthly_reports"} {
		var planted int64
		w.db.Table(table).Where("bujp_id = ? AND period IN ('2021-02','2021-03')", w.B.bujp.ID).Count(&planted)
		if planted > 0 {
			t.Errorf("PLANTED  %d %s row(s) for company B from a tenant user", planted, table)
		}
	}
	var n int64
	w.db.Table("users").Where("role IN ('super_admin','admin') AND email LIKE ?", "%"+w.suffix+"%@iso.test").Where("email NOT LIKE 'super-%'").Count(&n)
	if n > 0 {
		t.Errorf("ESCALATED  %d platform-level user(s) were created by tenant users", n)
	}
}

func (w *world) knownIDs(x *tenant) []string { return x.ids }

// TestListsNeverContainAnotherCompany: every list/lookup/dashboard/export a tenant user can call.
func TestListsNeverContainAnotherCompany(t *testing.T) {
	w := setup(t)
	lists := []string{"/bujps", "/locations", "/shifts", "/personnels", "/personnel", "/users", "/salary-components", "/assignments",
		"/attendances?per_page=100", "/attendances/summary", "/attendance-corrections", "/attendance-corrections/pending", "/leaves", "/leaves/pending",
		"/overtimes", "/overtimes/pending", "/patrols", "/payrolls", "/monthly-reports", "/loan-products", "/loan-products/active", "/loans",
		"/loans/history", "/loans/statistics", "/loans/admin-summary", "/loan-approvals/pending", "/notifications", "/dashboard/summary",
		"/personnels/template/download"}
	for _, side := range [][2]*tenant{{w.A, w.B}, {w.B, w.A}} {
		attacker, victim := side[0], side[1]
		for _, who := range []actor{attacker.admin, attacker.supervisor, attacker.guard} {
			for _, path := range lists {
				r := w.do(who, "GET", path, nil)
				if hit := r.leaks(victim.ids); len(hit) > 0 {
					t.Errorf("LIST LEAKS  GET %s as %s of %s shows other company's data: %v", path, who.user.Role, attacker.tag, hit)
				}
				if r.status >= 500 {
					t.Errorf("SERVER ERROR  GET %s as %s: %d (%s)", path, who.user.Role, r.status, trim(r.body))
				}
			}
		}
	}
}

// TestPlatformEndpointsAreNotForTenantUsers: company data (BUJP) is a platform resource.
func TestPlatformEndpointsAreNotForTenantUsers(t *testing.T) {
	w := setup(t)
	for _, who := range []actor{w.A.supervisor, w.A.guard} {
		if r := w.do(who, "GET", "/bujps", nil); r.ok() && len(r.leaks(w.B.ids)) > 0 {
			t.Errorf("%s can list other companies: %s", who.user.Role, trim(r.body))
		}
	}
	for _, who := range []actor{w.A.admin, w.A.supervisor, w.A.guard} {
		r := w.do(who, "POST", "/bujps", map[string]interface{}{"code": "ZZ" + uuid.NewString()[:6], "name": "created by " + who.user.Role})
		if r.ok() {
			t.Errorf("%s created a company (status %d)", who.user.Role, r.status)
		}
	}
}

// TestSanity proves the checks above are not vacuous: legitimate access still works.
func TestSanity(t *testing.T) {
	w := setup(t)
	for _, c := range []struct {
		who  actor
		path string
	}{
		{w.A.admin, "/locations/" + w.A.location.ID.String()},
		{w.A.admin, "/personnels/" + w.A.personnel.ID.String()},
		{w.A.admin, "/payrolls/" + w.A.payroll.ID.String()},
		{w.A.guard, "/attendances/" + w.A.attendance.ID.String()},
		{w.super, "/locations/" + w.A.location.ID.String()},
		{w.super, "/locations/" + w.B.location.ID.String()},
		{w.super, "/users/" + w.B.admin.user.ID.String()},
	} {
		if r := w.do(c.who, "GET", c.path, nil); !r.ok() {
			t.Errorf("%s should be able to GET %s, got %d (%s)", c.who.user.Role, c.path, r.status, trim(r.body))
		}
	}
	if r := w.do(w.A.admin, "GET", "/locations", nil); !r.ok() || !strings.Contains(r.body, w.A.location.ID.String()) {
		t.Errorf("admin A must see own location in the list: %d", r.status)
	}
	if r := w.do(w.super, "GET", "/locations?per_page=1000", nil); !strings.Contains(r.body, w.B.location.ID.String()) || !strings.Contains(r.body, w.A.location.ID.String()) {
		t.Errorf("platform admin must see both companies' locations")
	}
}

func trim(s string) string {
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

// TestRoleEscalationInsideOneCompany: nobody can give themselves or others more power than their own role allows.
func TestRoleEscalationInsideOneCompany(t *testing.T) {
	w := setup(t)
	a := w.A
	other := w.actor("otheradmin", "company_admin", &a.bujp.ID)
	cases := []struct {
		name   string
		who    actor
		method string
		path   string
		body   interface{}
		table  string
		id     uuid.UUID
	}{
		{"company admin promotes a guard to company_admin", a.admin, "PUT", "/users/" + a.guard.user.ID.String(), map[string]interface{}{"role": "company_admin"}, "users", a.guard.user.ID},
		{"company admin promotes a guard to super_admin", a.admin, "PUT", "/users/" + a.guard.user.ID.String(), map[string]interface{}{"role": "super_admin"}, "users", a.guard.user.ID},
		{"company admin promotes themselves", a.admin, "PUT", "/users/" + a.admin.user.ID.String(), map[string]interface{}{"role": "super_admin"}, "users", a.admin.user.ID},
		{"company admin edits another company admin", a.admin, "PUT", "/users/" + other.user.ID.String(), map[string]interface{}{"full_name": "hijacked"}, "users", other.user.ID},
		{"company admin deletes another company admin", a.admin, "DELETE", "/users/" + other.user.ID.String(), nil, "users", other.user.ID},
		{"company admin deletes themselves", a.admin, "DELETE", "/users/" + a.admin.user.ID.String(), nil, "users", a.admin.user.ID},
		{"supervisor edits a guard", a.supervisor, "PUT", "/users/" + a.guard.user.ID.String(), map[string]interface{}{"full_name": "hijacked"}, "users", a.guard.user.ID},
		{"guard promotes themselves", a.guard, "PUT", "/users/" + a.guard.user.ID.String(), map[string]interface{}{"role": "super_admin"}, "users", a.guard.user.ID},
		{"guard reads another guard's account", a.guard, "GET", "/users/" + a.extraUser.user.ID.String(), nil, "", uuid.Nil},
		{"guard edits assignments", a.guard, "POST", "/assignments", map[string]interface{}{"personnel_id": a.personnel.ID.String(), "location_id": a.location.ID.String(), "start_date": "2024-01-01"}, "", uuid.Nil},
		{"guard validates a patrol", a.guard, "POST", "/patrols/" + a.patrol.ID.String() + "/validate", map[string]interface{}{"validation_status": "valid"}, "patrols", a.patrol.ID},
		{"guard decides a leave", a.guard, "POST", "/leaves/" + a.leave.ID.String() + "/approve", map[string]interface{}{"status": "approved"}, "leaves", a.leave.ID},
		{"guard generates payroll", a.guard, "POST", "/payrolls/generate", map[string]interface{}{"bujp_id": a.bujp.ID.String(), "period": "2021-05"}, "", uuid.Nil},
		{"guard generates a monthly report", a.guard, "POST", "/monthly-reports/generate", map[string]interface{}{"period": "2021-05"}, "", uuid.Nil},
		{"supervisor reads monthly reports", a.supervisor, "GET", "/monthly-reports", nil, "", uuid.Nil},
	}
	for _, c := range cases {
		before := ""
		if c.table != "" {
			before = w.snapshot(c.table, c.id)
		}
		r := w.do(c.who, c.method, c.path, c.body)
		if r.ok() {
			t.Errorf("ALLOWED       %s: status %d (%s)", c.name, r.status, trim(r.body))
		}
		if c.table != "" && w.snapshot(c.table, c.id) != before {
			t.Errorf("DATA CHANGED  %s", c.name)
		}
	}
	// the legitimate counterparts still work (so the checks above are not vacuous)
	if r := w.do(a.admin, "POST", "/users", map[string]interface{}{"email": "new-" + w.suffix + "@iso.test", "password": "Passw0rd!x", "role": "guard", "full_name": "new guard"}); !r.ok() {
		t.Errorf("a company admin must still be able to create a guard in their own company: %d %s", r.status, trim(r.body))
	}
	if r := w.do(a.admin, "PUT", "/users/"+a.guard.user.ID.String(), map[string]interface{}{"full_name": "renamed"}); !r.ok() {
		t.Errorf("a company admin must still be able to edit a guard in their company: %d %s", r.status, trim(r.body))
	}
}

// TestStoredFilesAreTenantScoped: signed download links, moves and deletes of another company's files.
func TestStoredFilesAreTenantScoped(t *testing.T) {
	w := setup(t)
	a, b := w.A, w.B
	keyOf := func(folder string, id uuid.UUID) string { return folder + "/" + id.String() + "/photo/abc.jpg" }
	foreign := []string{
		keyOf("PERSONNEL/KTP", b.personnel.ID), keyOf("PERSONNELS", b.personnel.ID), keyOf("LOANS", b.guard.user.ID),
		keyOf("ATTENDANCES", b.guard.user.ID), keyOf("BUJPS", b.bujp.ID), keyOf("temp/PERSONNELS", b.personnel.ID),
	}
	for _, who := range []actor{a.admin, a.supervisor, a.guard} {
		for _, key := range foreign {
			if r := w.do(who, "POST", "/uploads/presigned-download-url", map[string]interface{}{"object_key": key}); r.ok() {
				t.Errorf("ALLOWED  %s can get a download link for %s", who.user.Role, key)
			}
			if r := w.do(who, "DELETE", "/uploads", map[string]interface{}{"object_key": key}); r.ok() {
				t.Errorf("ALLOWED  %s can delete %s", who.user.Role, key)
			}
			if r := w.do(who, "POST", "/uploads/move", map[string]interface{}{"source_key": key, "dest_key": keyOf("PERSONNELS", a.personnel.ID)}); r.ok() {
				t.Errorf("ALLOWED  %s can move %s into their own folder", who.user.Role, key)
			}
			if r := w.do(who, "POST", "/uploads/move", map[string]interface{}{"source_key": "temp/x.jpg", "dest_key": key}); r.ok() {
				t.Errorf("ALLOWED  %s can write into %s", who.user.Role, key)
			}
		}
		for _, folder := range []string{"PERSONNELS", "LOANS", "BUJPS"} {
			r := w.do(who, "POST", "/uploads/presigned-url", map[string]interface{}{"filename": "x.jpg", "folder_type": folder, "entity_id": b.personnel.ID.String()})
			if r.ok() {
				t.Errorf("ALLOWED  %s can get an upload link into company B's %s folder", who.user.Role, folder)
			}
		}
	}
	// legitimate use still works: own company's file for the admin, own files for the guard
	if r := w.do(a.admin, "POST", "/uploads/presigned-download-url", map[string]interface{}{"object_key": keyOf("PERSONNEL/KTP", a.personnel.ID)}); !r.ok() {
		t.Errorf("a company admin must reach their own company's files: %d %s", r.status, trim(r.body))
	}
	if r := w.do(a.guard, "POST", "/uploads/presigned-download-url", map[string]interface{}{"object_key": keyOf("ATTENDANCES", a.guard.user.ID)}); !r.ok() {
		t.Errorf("a guard must reach their own files: %d %s", r.status, trim(r.body))
	}
	if r := w.do(a.guard, "POST", "/uploads/presigned-download-url", map[string]interface{}{"object_key": keyOf("ATTENDANCES", a.extraUser.user.ID)}); r.ok() {
		t.Errorf("a guard must not reach a colleague's files")
	}
	if r := w.do(a.guard, "POST", "/uploads/presigned-download-url", map[string]interface{}{"object_key": "users/abc.jpg"}); !r.ok() {
		t.Errorf("shared direct-upload folders stay usable: %d %s", r.status, trim(r.body))
	}
}

// TestSwitchedOffAccountsAndChangedRolesApplyAtOnce: the token alone is not enough.
func TestSwitchedOffAccountsAndChangedRolesApplyAtOnce(t *testing.T) {
	w := setup(t)
	a := w.A
	if r := w.do(a.guard, "GET", "/locations", nil); !r.ok() {
		t.Fatalf("an active guard must be served: %d", r.status)
	}
	if err := w.db.Table("users").Where("id = ?", a.guard.user.ID).Update("status", "inactive").Error; err != nil {
		t.Fatal(err)
	}
	if r := w.do(a.guard, "GET", "/locations", nil); r.status != 401 {
		t.Errorf("a deactivated account kept access (status %d)", r.status)
	}
	// demote the company admin to guard in the database: the old admin token must stop working as admin
	if err := w.db.Table("users").Where("id = ?", a.admin.user.ID).Update("role", "guard").Error; err != nil {
		t.Fatal(err)
	}
	if r := w.do(a.admin, "POST", "/locations", map[string]interface{}{"code": "D" + w.suffix, "name": "x", "address": "x", "latitude": -6.2, "longitude": 106.8}); r.ok() {
		t.Errorf("a demoted admin could still create master data with the old token (status %d)", r.status)
	}
}
