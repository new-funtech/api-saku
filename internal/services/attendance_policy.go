package services

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // the zone must resolve even on an image without system tzdata

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

// Errors the handler maps to a status code (see handlers/errors.go).
var (
	ErrOutsideGeofence     = errors.New("outside the attendance radius")
	ErrCoordinatesRequired = errors.New("coordinates are required")
	ErrAlreadyCheckedOut   = errors.New("attendance already checked out")
	ErrNoOpenAttendance    = errors.New("no attendance record to check out; please check-in first")
)

var (
	attendanceLocOnce sync.Once
	attendanceLoc     *time.Location
)

// attendanceLocation is the time zone that defines "today" for attendance (APP_TIMEZONE, default
// Asia/Jakarta). The day must not depend on the server's own zone or on UTC.
func attendanceLocation() *time.Location {
	attendanceLocOnce.Do(func() {
		name := strings.TrimSpace(os.Getenv("APP_TIMEZONE"))
		if name == "" {
			name = "Asia/Jakarta"
		}
		loc, err := time.LoadLocation(name)
		if err != nil {
			log.Printf("[attendance] unknown APP_TIMEZONE %q (%v); using the server's local zone", name, err)
			loc = time.Local
		}
		attendanceLoc = loc
	})
	return attendanceLoc
}

// attendanceDay is the calendar day of `now` in the attendance zone, as the date value stored in
// attendances.date (midnight UTC of that date).
//
// time.Now().Truncate(24h) is NOT this: it cuts at midnight UTC, which is 07:00 WIB, so between
// 00:00 and 07:00 WIB it yields the previous day.
func attendanceDay(now time.Time) time.Time {
	n := now.In(attendanceLocation())
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// shiftClock puts a shift's time of day onto a date, in the attendance zone.
func shiftClock(date time.Time, tod time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), tod.Hour(), tod.Minute(), tod.Second(), 0, attendanceLocation())
}

// isOvernight reports a shift that ends on the next day (end not after start, e.g. 22:00-06:00).
func isOvernight(start, end time.Time) bool {
	return !start.IsZero() && !end.IsZero() && !end.After(start)
}

// shiftEnd is when the shift that started on `date` ends.
func shiftEnd(date time.Time, start, end time.Time) time.Time {
	e := shiftClock(date, end)
	if isOvernight(start, end) {
		e = e.Add(24 * time.Hour)
	}
	return e
}

// geofenceError carries the sentence shown to the employee and still matches ErrOutsideGeofence.
type geofenceError struct{ msg string }

func (e geofenceError) Error() string { return e.msg }
func (e geofenceError) Unwrap() error { return ErrOutsideGeofence }

const earthRadiusMeters = 6371000.0

// distanceMeters is the great-circle distance between two coordinates (haversine).
func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// geofenceToleranceMeters absorbs GPS rounding between what the phone checked and what the server
// recomputes. It is not a GPS-accuracy allowance.
const geofenceToleranceMeters = 5

// geofenceEnabled is on unless ATTENDANCE_GEOFENCE_ENABLED is false/0/off.
func geofenceEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ATTENDANCE_GEOFENCE_ENABLED"))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// checkGeofence verifies the reported coordinates are inside the office radius. A location with no
// coordinates set cannot be verified and is let through.
func checkGeofence(loc *model.Location, lat, lng model.NumberString) error {
	if !geofenceEnabled() || loc == nil || loc.Latitude == nil || loc.Longitude == nil {
		return nil
	}
	la, err1 := strconv.ParseFloat(strings.TrimSpace(string(lat)), 64)
	lo, err2 := strconv.ParseFloat(strings.TrimSpace(string(lng)), 64)
	if err1 != nil || err2 != nil || (la == 0 && lo == 0) {
		return ErrCoordinatesRequired
	}
	radius := loc.AttendanceRadius
	if radius <= 0 {
		radius = 100
	}
	d := distanceMeters(la, lo, *loc.Latitude, *loc.Longitude)
	if d > float64(radius)+geofenceToleranceMeters {
		return geofenceError{fmt.Sprintf("Anda berada sekitar %d m dari %s, batas radius absen %d m", int(math.Round(d)), loc.Name, radius)}
	}
	return nil
}

// idempotencyNamespace makes the derived keys stable across restarts and instances.
var idempotencyNamespace = uuid.MustParse("6f1d2c1a-5b0e-4f55-9a53-2a7e8b9c1d44")

// deriveIdempotencyKey is the key used when the client sends none: one per employee, day and action,
// so a plain retry of the same action is still recognised.
func deriveIdempotencyKey(personnelID uuid.UUID, day time.Time, action string) string {
	return uuid.NewSHA1(idempotencyNamespace, []byte(personnelID.String()+"|"+day.Format("2006-01-02")+"|"+action)).String()
}
