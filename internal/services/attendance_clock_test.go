package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
)

// --- fakes: only what the clock-in/out path calls ---

type fakeAttendanceRepo struct {
	repository.AttendanceRepository
	rows      map[uuid.UUID]*model.Attendance
	inserted  []*model.Attendance
	calls     int
	completed int
}

func newFakeAttendanceRepo() *fakeAttendanceRepo {
	return &fakeAttendanceRepo{rows: map[uuid.UUID]*model.Attendance{}}
}

func (f *fakeAttendanceRepo) ExistsByPersonnelAndDate(_ context.Context, p uuid.UUID, d time.Time) (bool, error) {
	f.calls++
	for _, r := range f.rows {
		if r.PersonnelID == p && r.Date.Equal(d) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAttendanceRepo) Insert(_ context.Context, a *model.Attendance) error {
	f.calls++
	a.ID = uuid.New()
	f.rows[a.ID] = a
	f.inserted = append(f.inserted, a)
	return nil
}

func (f *fakeAttendanceRepo) FindOpenForCheckout(_ context.Context, p uuid.UUID, from, to time.Time) ([]model.Attendance, error) {
	f.calls++
	var out []model.Attendance
	for _, r := range f.rows {
		if r.PersonnelID == p && !r.Date.Before(from) && !r.Date.After(to) {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeAttendanceRepo) CompleteCheckOut(_ context.Context, id uuid.UUID, c repository.CheckOutUpdate) (bool, error) {
	f.calls++
	r := f.rows[id]
	if r == nil || r.CheckOut != nil {
		return false, nil
	}
	at := c.At
	r.CheckOut = &at
	f.completed++
	return true, nil
}

type fakeAssignmentRepo struct {
	repository.AssignmentRepository
	assignment *model.Assignment
	calls      int
}

func (f *fakeAssignmentRepo) FindActiveForAttendance(context.Context, uuid.UUID, time.Time) (*model.Assignment, error) {
	f.calls++
	if f.assignment == nil {
		return nil, errors.New("not found")
	}
	return f.assignment, nil
}

func tod(h, m int) model.TimeOnly {
	return model.TimeOnly{Time: time.Date(0, 1, 1, h, m, 0, 0, time.UTC)}
}

func officeAssignment(start, end model.TimeOnly) *model.Assignment {
	lat, lng := -6.2, 106.816666
	loc := &model.Location{ID: uuid.New(), Name: "Kantor", Latitude: &lat, Longitude: &lng, AttendanceRadius: 100}
	return &model.Assignment{
		ID: uuid.New(), LocationID: loc.ID, Location: loc,
		Shift: &model.Shift{StartTime: start, EndTime: end},
	}
}

func newClockService(a *model.Assignment) (*attendanceServiceImpl, *fakeAttendanceRepo, *fakeAssignmentRepo) {
	ar, asg := newFakeAttendanceRepo(), &fakeAssignmentRepo{assignment: a}
	return &attendanceServiceImpl{repo: ar, assignmentRepo: asg}, ar, asg
}

func atOffice() *model.CreateAttendanceRequest {
	return &model.CreateAttendanceRequest{CheckInLatitude: "-6.2", CheckInLongitude: "106.816666"}
}

func TestClockInIgnoresWhatTheClientSaysAboutTimeAndStatus(t *testing.T) {
	// Shift started a minute after midnight; the real "now" is always later than that + grace, so it is late.
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	forged := "2020-01-01T00:00:00+07:00"
	req := atOffice()
	req.Date, req.CheckIn, req.CheckOut, req.Status = "2000-01-01", &forged, &forged, "present"

	resp, err := svc.ClockIn(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatal(err)
	}
	row := repo.inserted[0]
	if row.Status != "late" {
		t.Errorf("status = %q, the client's \"present\" must be ignored", row.Status)
	}
	if row.CheckOut != nil {
		t.Errorf("a clock-in must not carry a check-out")
	}
	if got := time.Since(*row.CheckIn); got < 0 || got > time.Minute {
		t.Errorf("check_in = %v, want the server's current time", row.CheckIn)
	}
	if !row.Date.Equal(attendanceDay(time.Now())) {
		t.Errorf("date = %v, want today", row.Date)
	}
	if resp.Status != "late" {
		t.Errorf("response status = %q", resp.Status)
	}
}

func TestClockInUsesFewQueries(t *testing.T) {
	svc, repo, asg := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	if _, err := svc.ClockIn(context.Background(), uuid.New(), atOffice()); err != nil {
		t.Fatal(err)
	}
	// assignment (office+shift in one) + duplicate check + insert = 3; the scope lookup is cached upstream.
	if total := asg.calls + repo.calls; total > 3 {
		t.Errorf("%d queries for a clock-in, want at most 3 (scope lookups come on top, at most 2 on a cache miss)", total)
	}
}

func TestClockInRejectsADuplicateDay(t *testing.T) {
	svc, _, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	p := uuid.New()
	if _, err := svc.ClockIn(context.Background(), p, atOffice()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClockIn(context.Background(), p, atOffice()); err == nil {
		t.Fatal("a second clock-in the same day must be refused")
	}
}

func TestGeofence(t *testing.T) {
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	far := &model.CreateAttendanceRequest{CheckInLatitude: "-6.21", CheckInLongitude: "106.816666"} // ~1.1 km away
	if _, err := svc.ClockIn(context.Background(), uuid.New(), far); !errors.Is(err, ErrOutsideGeofence) {
		t.Fatalf("far away: err = %v, want ErrOutsideGeofence", err)
	}
	if _, err := svc.ClockIn(context.Background(), uuid.New(), &model.CreateAttendanceRequest{}); !errors.Is(err, ErrCoordinatesRequired) {
		t.Fatalf("no coordinates: err = %v, want ErrCoordinatesRequired", err)
	}
	near := &model.CreateAttendanceRequest{CheckInLatitude: "-6.2005", CheckInLongitude: "106.816666"} // ~55 m
	if _, err := svc.ClockIn(context.Background(), uuid.New(), near); err != nil {
		t.Fatalf("inside the radius: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Errorf("rows = %d, want only the one inside the radius", len(repo.inserted))
	}
}

func TestGeofenceCanBeSwitchedOffAndSkipsOfficesWithoutCoordinates(t *testing.T) {
	t.Setenv("ATTENDANCE_GEOFENCE_ENABLED", "false")
	svc, _, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	if _, err := svc.ClockIn(context.Background(), uuid.New(), &model.CreateAttendanceRequest{}); err != nil {
		t.Fatalf("geofence off: %v", err)
	}
	t.Setenv("ATTENDANCE_GEOFENCE_ENABLED", "")
	a := officeAssignment(tod(0, 1), tod(23, 59))
	a.Location.Latitude, a.Location.Longitude = nil, nil
	svc2, _, _ := newClockService(a)
	if _, err := svc2.ClockIn(context.Background(), uuid.New(), &model.CreateAttendanceRequest{}); err != nil {
		t.Fatalf("office without coordinates cannot be verified and must pass: %v", err)
	}
}

func openRow(p uuid.UUID, a *model.Assignment) *model.Attendance {
	in := time.Now().Add(-3 * time.Hour)
	return &model.Attendance{
		ID: uuid.New(), PersonnelID: p, LocationID: a.LocationID, AssignmentID: &a.ID,
		Date: attendanceDay(time.Now()), CheckIn: &in, Location: a.Location, Assignment: a,
	}
}

func TestClockOutDoesNotOverwriteTheFirst(t *testing.T) {
	a := officeAssignment(tod(0, 0), tod(0, 1)) // shift already over
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row

	out := &model.UpdateAttendanceRequest{CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"}
	if _, err := svc.ClockOut(context.Background(), p, out); err != nil {
		t.Fatal(err)
	}
	first := *repo.rows[row.ID].CheckOut
	if _, err := svc.ClockOut(context.Background(), p, out); !errors.Is(err, ErrAlreadyCheckedOut) {
		t.Fatalf("second clock-out: err = %v, want ErrAlreadyCheckedOut", err)
	}
	if !repo.rows[row.ID].CheckOut.Equal(first) || repo.completed != 1 {
		t.Errorf("the first check-out was overwritten")
	}
}

func TestClockOutIgnoresTheClientTime(t *testing.T) {
	a := officeAssignment(tod(0, 0), tod(0, 1))
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row
	forged := "2031-01-01T23:00:00+07:00"
	req := &model.UpdateAttendanceRequest{CheckOut: &forged, CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"}
	if _, err := svc.ClockOut(context.Background(), p, req); err != nil {
		t.Fatal(err)
	}
	if got := repo.rows[row.ID].CheckOut; time.Since(*got) > time.Minute || time.Since(*got) < 0 {
		t.Errorf("check_out = %v, want the server's time", got)
	}
}

func TestClockOutBeforeTheShiftEnds(t *testing.T) {
	// A shift that ends at 23:59 cannot be closed any earlier in the day (this test runs earlier except in the last minute).
	if n := time.Now().In(attendanceLocation()); n.Hour() == 23 && n.Minute() >= 58 {
		t.Skip("too close to the end of the shift")
	}
	a := officeAssignment(tod(0, 0), tod(23, 59))
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row
	req := &model.UpdateAttendanceRequest{CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"}
	if _, err := svc.ClockOut(context.Background(), p, req); err == nil {
		t.Fatal("must refuse a check-out before the shift ends")
	}
}

func TestClockOutWithoutClockIn(t *testing.T) {
	svc, _, _ := newClockService(officeAssignment(tod(0, 0), tod(0, 1)))
	if _, err := svc.ClockOut(context.Background(), uuid.New(), &model.UpdateAttendanceRequest{}); !errors.Is(err, ErrNoOpenAttendance) {
		t.Fatalf("err = %v, want ErrNoOpenAttendance", err)
	}
}
