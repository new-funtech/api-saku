package services

import (
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

func wib(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, attendanceLocation())
}

// Between 00:00 and 07:00 WIB the UTC date is still the previous day. The old
// time.Now().Truncate(24h) therefore pointed at yesterday, so a check-out in those hours looked up
// the wrong row. attendanceDay follows the local calendar day.
func TestAttendanceDayAroundMidnightWIB(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"00:00 WIB", wib(2026, 11, 14, 0, 0), "2026-11-14"},
		{"00:30 WIB", wib(2026, 11, 14, 0, 30), "2026-11-14"},
		{"02:00 WIB", wib(2026, 11, 14, 2, 0), "2026-11-14"},
		{"06:59 WIB", wib(2026, 11, 14, 6, 59), "2026-11-14"},
		{"07:00 WIB", wib(2026, 11, 14, 7, 0), "2026-11-14"},
		{"23:59 WIB", wib(2026, 11, 14, 23, 59), "2026-11-14"},
		{"same instant expressed in UTC", wib(2026, 11, 14, 2, 0).UTC(), "2026-11-14"},
	}
	for _, c := range cases {
		if got := attendanceDay(c.at).Format("2006-01-02"); got != c.want {
			t.Errorf("%s: day = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestTruncateWasWrongBetweenMidnightAndSevenWIB(t *testing.T) {
	at := wib(2026, 11, 14, 2, 0)
	old := at.UTC().Truncate(24 * time.Hour).Format("2006-01-02") // what the code used to compute
	if old == attendanceDay(at).Format("2006-01-02") {
		t.Fatalf("expected the old calculation to differ at 02:00 WIB, both gave %s", old)
	}
}

func TestOvernightShiftCheckOutAfterMidnight(t *testing.T) {
	start, end := tod(22, 0), tod(6, 0)
	day := attendanceDay(wib(2026, 11, 14, 23, 0)) // clocked in on the 14th at 23:00

	if !isOvernight(start.Time, end.Time) {
		t.Fatal("22:00-06:00 is an overnight shift")
	}
	if got, want := shiftEnd(day, start.Time, end.Time), wib(2026, 11, 15, 6, 0); !got.Equal(want) {
		t.Errorf("shift end = %v, want %v", got, want)
	}

	a := &model.Assignment{ID: uuid.New(), Shift: &model.Shift{StartTime: start, EndTime: end}}
	open := []model.Attendance{{ID: uuid.New(), Date: day, Assignment: a}}

	// At 06:30 on the 15th the guard checks out: "today" is the 15th, the row is the 14th's.
	today := attendanceDay(wib(2026, 11, 15, 6, 30))
	if pickAttendanceToCheckOut(open, today) == nil {
		t.Error("an open overnight attendance from yesterday must be found")
	}

	// A day shift left open yesterday is not picked up today.
	dayShift := &model.Assignment{ID: uuid.New(), Shift: &model.Shift{StartTime: tod(8, 0), EndTime: tod(17, 0)}}
	if pickAttendanceToCheckOut([]model.Attendance{{ID: uuid.New(), Date: day, Assignment: dayShift}}, today) != nil {
		t.Error("a day-shift attendance from yesterday must not be reused")
	}
}

func TestLateUsesTheAttendanceZone(t *testing.T) {
	day := attendanceDay(wib(2026, 11, 14, 8, 30))
	start := tod(8, 0).Time
	if !isLateCheckIn(day, start, wib(2026, 11, 14, 8, 30), attendanceLateGraceMinutes) {
		t.Error("08:30 WIB against an 08:00 shift is late")
	}
	if isLateCheckIn(day, start, wib(2026, 11, 14, 8, 4), attendanceLateGraceMinutes) {
		t.Error("08:04 WIB is inside the grace period")
	}
	// The same instant in UTC (01:30) must give the same answer.
	if !isLateCheckIn(day, start, wib(2026, 11, 14, 8, 30).UTC(), attendanceLateGraceMinutes) {
		t.Error("lateness must not depend on the zone the instant is written in")
	}
}

func TestDistanceMeters(t *testing.T) {
	if d := distanceMeters(-6.2, 106.816666, -6.2, 106.816666); d > 0.01 {
		t.Errorf("same point = %v m", d)
	}
	// 0.001 degree of latitude is about 111 m.
	if d := distanceMeters(-6.2, 106.8, -6.201, 106.8); d < 105 || d > 117 {
		t.Errorf("0.001 deg = %v m, want about 111", d)
	}
}
