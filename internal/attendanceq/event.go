// Package attendanceq is the asynchronous path of clock-in/clock-out: the API validates and
// publishes an Event, a separate worker stores it. It is opt-in (ATTENDANCE_ASYNC=true); the
// synchronous path in services.AttendanceService stays the default.
package attendanceq

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Action string

const (
	ActionCheckIn  Action = "checkin"
	ActionCheckOut Action = "checkout"
)

// Event is one clock-in or clock-out accepted by the API.
//
// The official time is ServerReceivedAt, set by the API when it accepted the request. The worker
// may run seconds or minutes later, so its own clock never decides anything. ClientTimestamp is
// only what the device claimed, kept for audit.
type Event struct {
	Version          int        `json:"v"`
	Action           Action     `json:"action"`
	Key              string     `json:"idempotency_key"`
	RequestID        string     `json:"request_id,omitempty"`
	PersonnelID      uuid.UUID  `json:"personnel_id"`
	LocationID       uuid.UUID  `json:"location_id"`
	AssignmentID     *uuid.UUID `json:"assignment_id,omitempty"`
	Day              string     `json:"day"` // the attendance date (YYYY-MM-DD) decided by the API
	Status           string     `json:"status,omitempty"`
	Latitude         string     `json:"latitude,omitempty"`
	Longitude        string     `json:"longitude,omitempty"`
	Photo            string     `json:"photo,omitempty"` // base64 data URL; the worker uploads it
	Document         string     `json:"document,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	ClientTimestamp  *time.Time `json:"client_timestamp,omitempty"`
	ServerReceivedAt time.Time  `json:"server_received_at"`
}

// Enqueuer hands an event to the broker. It returns only after the broker confirmed the message, so
// a nil error means the event is durably queued; an error means it is not and the client must be told.
type Enqueuer interface {
	Enqueue(ctx context.Context, ev Event) error
}

// Config is read from the environment by LoadConfig.
type Config struct {
	Queue        string
	RetryDelays  []time.Duration
	MaxAttempts  int
	Prefetch     int
	BatchSize    int
	BatchWait    time.Duration
	PublishPool  int
	PublishWait  time.Duration
	MetricsAddr  string
	MaxPhotoSize int
}
