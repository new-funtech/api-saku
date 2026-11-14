package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/attendanceq"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

// ErrQueueUnavailable means the broker did not confirm the event. Nothing was queued, so the
// client must be told and may retry with the same Idempotency-Key.
var ErrQueueUnavailable = errors.New("attendance queue unavailable")

// AsyncAccepted is the 202 body: the request was validated and durably queued, not yet stored.
type AsyncAccepted struct {
	IdempotencyKey string    `json:"idempotency_key"`
	State          string    `json:"state"` // always "pending" here
	ReceivedAt     time.Time `json:"received_at"`
	StatusURL      string    `json:"status_url"`
}

// QueueStatus is the answer of the status endpoint.
type QueueStatus struct {
	IdempotencyKey string     `json:"idempotency_key"`
	State          string     `json:"state"` // pending | recorded | failed
	Action         string     `json:"action,omitempty"`
	AttendanceID   *uuid.UUID `json:"attendance_id,omitempty"`
	Error          string     `json:"error,omitempty"`
}

func (s *attendanceServiceImpl) SetEnqueuer(e attendanceq.Enqueuer) { s.enqueuer = e }

// AsyncEnabled is true when the feature flag is on AND a broker publisher is attached; otherwise
// every request takes the synchronous path.
func (s *attendanceServiceImpl) AsyncEnabled() bool {
	return s.enqueuer != nil && attendanceq.AsyncEnabled()
}

func statusURL(key string) string { return "/api/v1/attendances/status/" + key }

func clientTimestamp(raw *string) *time.Time {
	if raw == nil {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw)); err == nil {
		return &t
	}
	return nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ClockInAsync does the cheap validation (active assignment, geofence) and publishes. The official
// time is the moment of this call, not when the worker runs.
func (s *attendanceServiceImpl) ClockInAsync(ctx context.Context, personnelID uuid.UUID, req *model.CreateAttendanceRequest) (*AsyncAccepted, error) {
	if personnelID == uuid.Nil {
		return nil, errors.New("personnel record for current user not found")
	}
	now := time.Now()
	day := attendanceDay(now)
	assignment, err := s.assignmentRepo.FindActiveForAttendance(ctx, personnelID, day)
	if err != nil || assignment == nil {
		return nil, errors.New("no active assignment for current user")
	}
	if err := checkGeofence(assignment.Location, req.CheckInLatitude, req.CheckInLongitude); err != nil {
		return nil, err
	}
	status := "present"
	if assignment.Shift != nil && !assignment.Shift.StartTime.IsZero() &&
		isLateCheckIn(day, assignment.Shift.StartTime.Time, now, attendanceLateGraceMinutes) {
		status = "late"
	}
	key := req.IdempotencyKey
	if key == "" {
		key = deriveIdempotencyKey(personnelID, day, "checkin")
	}
	ev := attendanceq.Event{
		Action: attendanceq.ActionCheckIn, Key: key, PersonnelID: personnelID, LocationID: assignment.LocationID,
		AssignmentID: &assignment.ID, Day: day.Format("2006-01-02"), Status: status,
		Latitude: string(req.CheckInLatitude), Longitude: string(req.CheckInLongitude),
		Photo: deref(req.CheckInPhoto), Document: deref(req.SupportingDocument), Notes: req.Notes,
		ClientTimestamp: clientTimestamp(req.CheckIn), ServerReceivedAt: now,
	}
	if err := s.enqueuer.Enqueue(ctx, ev); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	return &AsyncAccepted{IdempotencyKey: key, State: "pending", ReceivedAt: now, StatusURL: statusURL(key)}, nil
}

// ClockOutAsync validates against the assignment's shift and publishes. A check-out after midnight
// of an overnight shift belongs to the previous day's attendance.
func (s *attendanceServiceImpl) ClockOutAsync(ctx context.Context, personnelID uuid.UUID, req *model.UpdateAttendanceRequest) (*AsyncAccepted, error) {
	if personnelID == uuid.Nil {
		return nil, errors.New("personnel record for current user not found")
	}
	now := time.Now()
	day := attendanceDay(now)
	assignment, err := s.assignmentRepo.FindActiveForAttendance(ctx, personnelID, day)
	if err != nil || assignment == nil {
		return nil, errors.New("no active assignment for current user")
	}
	attDate := day
	if assignment.Shift != nil {
		start, end := assignment.Shift.StartTime.Time, assignment.Shift.EndTime.Time
		if isOvernight(start, end) && now.Before(shiftEnd(day.AddDate(0, 0, -1), start, end)) {
			attDate = day.AddDate(0, 0, -1)
		}
		if !end.IsZero() && now.Before(shiftEnd(attDate, start, end)) {
			return nil, fmt.Errorf("belum waktunya checkout, shift Anda berakhir pukul %s", end.Format("15:04"))
		}
	}
	if err := checkGeofence(assignment.Location, req.CheckOutLatitude, req.CheckOutLongitude); err != nil {
		return nil, err
	}
	key := req.IdempotencyKey
	if key == "" {
		key = deriveIdempotencyKey(personnelID, attDate, "checkout")
	}
	ev := attendanceq.Event{
		Action: attendanceq.ActionCheckOut, Key: key, PersonnelID: personnelID, LocationID: assignment.LocationID,
		AssignmentID: &assignment.ID, Day: attDate.Format("2006-01-02"),
		Latitude: string(req.CheckOutLatitude), Longitude: string(req.CheckOutLongitude),
		Photo: deref(req.CheckOutPhoto), Document: deref(req.SupportingDocument), Notes: req.Notes,
		ClientTimestamp: clientTimestamp(req.CheckOut), ServerReceivedAt: now,
	}
	if err := s.enqueuer.Enqueue(ctx, ev); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	return &AsyncAccepted{IdempotencyKey: key, State: "pending", ReceivedAt: now, StatusURL: statusURL(key)}, nil
}

// QueueStatus answers "did my request get stored?": recorded once a row carries the key, failed
// when the worker gave up, pending otherwise (still queued, or never accepted).
func (s *attendanceServiceImpl) QueueStatus(ctx context.Context, personnelID uuid.UUID, key string) (*QueueStatus, error) {
	st := &QueueStatus{IdempotencyKey: key, State: "pending"}
	att, action, err := s.repo.FindByIdempotencyKey(ctx, personnelID, key)
	if err != nil {
		return nil, err
	}
	if att != nil {
		st.State, st.Action, st.AttendanceID = "recorded", action, &att.ID
		return st, nil
	}
	f, err := s.repo.FindFailure(ctx, personnelID, key)
	if err != nil {
		return nil, err
	}
	if f != nil {
		st.State, st.Action, st.Error = "failed", f.Action, f.Error
	}
	return st, nil
}
