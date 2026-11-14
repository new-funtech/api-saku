// Package tenantiso is a black-box test of tenant isolation: it seeds two companies (BUJP A and B),
// then, signed in as company A's users, tries to read, change, delete and export company B's data
// over HTTP. Everything must be refused, and company B's rows must be unchanged afterwards.
//
// It needs a running API and a throwaway PostgreSQL, and only runs when told where they are:
//
//	TENANTISO_BASE_URL=http://127.0.0.1:4010 TENANTISO_DB=127.0.0.1:55432 go test ./tests/tenantiso/
//
// See tests/tenantiso/run.sh, which starts both against disposable containers.
package tenantiso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const testSecret = "loadtest-secret-not-for-production"

type actor struct {
	name  string
	user  model.User
	token string
}

type tenant struct {
	tag                       string
	bujp                      model.Bujp
	admin, supervisor, guard  actor
	personnel, extraPersonnel model.Personnel
	location                  model.Location
	shift                     model.Shift
	assignment                model.Assignment
	attendance                model.Attendance
	leave                     model.Leave
	correction                model.AttendanceCorrection
	overtime                  model.Overtime
	patrol                    model.Patrol
	component                 model.SalaryComponent
	payroll                   model.Payroll
	report                    model.MonthlyReport
	product                   model.LoanProduct
	loan                      model.Loan
	approval                  model.LoanApproval
	notification              model.Notification
	extraUser                 actor    // a guard of the tenant without personnel
	ids                       []string // every identifier that must never appear in the other tenant's responses
}

type world struct {
	t      *testing.T
	base   string
	db     *gorm.DB
	A, B   *tenant
	super  actor
	suffix string
	client *http.Client
}

func setup(t *testing.T) *world {
	t.Helper()
	base, dbAddr := os.Getenv("TENANTISO_BASE_URL"), os.Getenv("TENANTISO_DB")
	if base == "" || dbAddr == "" {
		t.Skip("set TENANTISO_BASE_URL and TENANTISO_DB (see tests/tenantiso/run.sh)")
	}
	host, port := dbAddr, "5432"
	if i := strings.LastIndex(dbAddr, ":"); i > 0 {
		host, port = dbAddr[:i], dbAddr[i+1:]
	}
	dsn := fmt.Sprintf("host=%s port=%s user=postgres password=loadtest dbname=hrmis_loadtest sslmode=disable", host, port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	w := &world{t: t, base: strings.TrimRight(base, "/"), db: db, suffix: uuid.NewString()[:6], client: &http.Client{Timeout: 20 * time.Second}}
	w.A, w.B = w.seed("A"), w.seed("B")
	w.super = w.actor("super", "super_admin", nil)
	return w
}

func (w *world) token(u model.User) string {
	claims := jwtlib.MapClaims{"user_id": u.ID.String(), "email": u.Email, "role": u.Role, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()}
	s, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		w.t.Fatal(err)
	}
	return s
}

func (w *world) must(v interface{}) {
	w.t.Helper()
	if err := w.db.Create(v).Error; err != nil {
		w.t.Fatalf("seed %T: %v", v, err)
	}
}

func (w *world) actor(name, role string, bujp *uuid.UUID) actor {
	u := model.User{ID: uuid.New(), BujpID: bujp, Email: fmt.Sprintf("%s-%s@iso.test", name, w.suffix), Password: "x", Role: role, FullName: name, Status: "active"}
	w.must(&u)
	return actor{name: name, user: u, token: w.token(u)}
}

func f(v float64) *float64 { return &v }

func (w *world) seed(tag string) *tenant {
	s := w.suffix + tag
	d := &tenant{tag: tag}
	d.bujp = model.Bujp{ID: uuid.New(), Code: "I" + s, Name: "Company " + tag + " " + w.suffix}
	w.must(&d.bujp)
	b := d.bujp.ID
	d.admin = w.actor("admin"+tag, "company_admin", &b)
	d.supervisor = w.actor("sup"+tag, "supervisor", &b)
	d.guard = w.actor("guard"+tag, "guard", &b)
	d.extraUser = w.actor("extra"+tag, "guard", &b)

	d.personnel = model.Personnel{ID: uuid.New(), BujpID: b, UserID: &d.guard.user.ID, IDNumber: "P" + s, FullName: "Guard " + tag, JoinDate: time.Now(), Status: "active"}
	w.must(&d.personnel)
	d.extraPersonnel = model.Personnel{ID: uuid.New(), BujpID: b, IDNumber: "Q" + s, FullName: "Extra " + tag, JoinDate: time.Now(), Status: "active"}
	w.must(&d.extraPersonnel)
	// the supervisor is a person too (some endpoints resolve the caller's personnel)
	sp := model.Personnel{ID: uuid.New(), BujpID: b, UserID: &d.supervisor.user.ID, IDNumber: "S" + s, FullName: "Sup " + tag, JoinDate: time.Now(), Status: "active"}
	w.must(&sp)

	d.location = model.Location{ID: uuid.New(), BujpID: b, Code: "L" + s, Name: "Location " + tag, Address: "Jl " + tag, Latitude: f(-6.2), Longitude: f(106.8), AttendanceRadius: 100, Status: "active"}
	w.must(&d.location)
	d.shift = model.Shift{ID: uuid.New(), BujpID: &b, Name: "Shift " + tag + w.suffix, StartTime: model.TimeOnly{Time: time.Date(0, 1, 1, 8, 0, 0, 0, time.UTC)}, EndTime: model.TimeOnly{Time: time.Date(0, 1, 1, 16, 0, 0, 0, time.UTC)}}
	w.must(&d.shift)
	d.assignment = model.Assignment{ID: uuid.New(), PersonnelID: d.personnel.ID, LocationID: d.location.ID, ShiftID: &d.shift.ID, StartDate: time.Now().AddDate(0, 0, -5), Status: "active"}
	w.must(&d.assignment)

	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
	now := time.Now()
	d.attendance = model.Attendance{ID: uuid.New(), PersonnelID: d.personnel.ID, LocationID: d.location.ID, AssignmentID: &d.assignment.ID, Date: today, CheckIn: &now, Status: "present"}
	w.must(&d.attendance)
	d.leave = model.Leave{ID: uuid.New(), PersonnelID: d.personnel.ID, Type: "leave", StartDate: today.AddDate(0, 0, 3), EndDate: today.AddDate(0, 0, 4), Reason: "iso " + tag, Status: "pending"}
	w.must(&d.leave)
	d.correction = model.AttendanceCorrection{ID: uuid.New(), PersonnelID: d.personnel.ID, CorrectionDate: today.AddDate(0, 0, -1), CorrectionType: "checkin", Reason: "iso " + tag, Status: "pending"}
	w.must(&d.correction)
	d.overtime = model.Overtime{ID: uuid.New(), PersonnelID: d.personnel.ID, Date: today.AddDate(0, 0, -1), StartTime: "17:00", EndTime: "19:00", TotalHours: 2, Reason: "iso " + tag, Status: "pending"}
	w.must(&d.overtime)
	d.patrol = model.Patrol{ID: uuid.New(), PersonnelID: d.personnel.ID, LocationID: d.location.ID, Date: today, PatrolTime: now}
	w.must(&d.patrol)
	d.component = model.SalaryComponent{ID: uuid.New(), BujpID: &b, Code: "C" + s, Name: "Component " + tag, Type: "allowance", CalculationMethod: "fixed", Amount: 1000, IsActive: true}
	w.must(&d.component)
	d.payroll = model.Payroll{ID: uuid.New(), PersonnelID: d.personnel.ID, BujpID: b, Period: "2020-01", BaseSalary: 1000, NetSalary: 1000, PaymentStatus: "pending"}
	w.must(&d.payroll)
	d.report = model.MonthlyReport{ID: uuid.New(), BujpID: b, Period: "2020-01"}
	w.must(&d.report)
	d.product = model.LoanProduct{ID: uuid.New(), BujpID: &b, Code: "LP" + s, Name: "Product " + tag, IsActive: true}
	w.must(&d.product)
	d.loan = model.Loan{ID: uuid.New(), LoanNumber: "LN" + s, PersonnelID: d.personnel.ID, BujpID: &b, Purpose: "iso", LoanAmount: 1000000, InterestRate: 1, TenorMonths: 6, MonthlyInstallment: 200000, TotalRepayment: 1200000, Status: "pending_bujp"}
	w.must(&d.loan)
	d.approval = model.LoanApproval{ID: uuid.New(), LoanID: d.loan.ID, ApprovalLevel: 1, ApprovalLevelName: "BUJP", Status: "pending"}
	w.must(&d.approval)
	d.notification = model.Notification{ID: uuid.New(), RecipientUserID: d.guard.user.ID, BujpID: &b, Type: "LOAN_SUBMITTED", Title: "iso " + tag, Body: "secret body " + tag, EntityType: "loan", EntityID: d.loan.ID, EventID: "iso-" + s}
	w.must(&d.notification)

	for _, id := range []uuid.UUID{b, d.admin.user.ID, d.supervisor.user.ID, d.guard.user.ID, d.extraUser.user.ID, d.personnel.ID, d.extraPersonnel.ID, sp.ID,
		d.location.ID, d.shift.ID, d.assignment.ID, d.attendance.ID, d.leave.ID, d.correction.ID, d.overtime.ID, d.patrol.ID, d.component.ID,
		d.payroll.ID, d.report.ID, d.product.ID, d.loan.ID, d.approval.ID, d.notification.ID} {
		d.ids = append(d.ids, id.String())
	}
	d.ids = append(d.ids, d.admin.user.Email, d.guard.user.Email, d.personnel.IDNumber, d.loan.LoanNumber, "secret body "+tag)
	return d
}

// --- HTTP ---

type reply struct {
	status int
	body   string
}

func (w *world) do(a actor, method, path string, body interface{}) reply {
	w.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, w.base+"/api/v1"+path, rd)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.client.Do(req)
	if err != nil {
		w.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return reply{resp.StatusCode, string(raw)}
}

func (r reply) ok() bool { return r.status >= 200 && r.status < 300 }

func (r reply) leaks(ids []string) []string {
	var hit []string
	for _, id := range ids {
		if strings.Contains(r.body, id) {
			hit = append(hit, id)
		}
	}
	return hit
}

// snapshot reads a row as a generic map so a change (or deletion) of another tenant's row is detected.
func (w *world) snapshot(table string, id uuid.UUID) string {
	var row map[string]interface{}
	err := w.db.Table(table).Where("id = ?", id).Take(&row).Error
	if err != nil {
		return "<missing>"
	}
	delete(row, "updated_at")
	raw, _ := json.Marshal(row)
	return string(raw)
}
