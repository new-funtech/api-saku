package model

import (
	"time"

	"github.com/google/uuid"
)

// AttendanceFailure records an asynchronous clock-in/out that could not be stored, so the status
// endpoint can answer "failed" and an operator can see why. Created by the attendance worker.
type AttendanceFailure struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID    uuid.UUID `json:"personnel_id" gorm:"type:uuid;not null"`
	IdempotencyKey string    `json:"idempotency_key" gorm:"type:varchar(64);not null"`
	Action         string    `json:"action" gorm:"type:varchar(10);not null"`
	Error          string    `json:"error" gorm:"type:text"`
	Attempts       int       `json:"attempts"`
	CreatedAt      time.Time `json:"created_at"`
}
