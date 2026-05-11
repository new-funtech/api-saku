package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Patrol struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID      uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null"`
	LocationID       uuid.UUID  `json:"location_id" gorm:"type:uuid;not null"`
	AttendanceID     *uuid.UUID `json:"attendance_id" gorm:"type:uuid"`
	Date             time.Time  `json:"date" gorm:"type:date;not null"`
	PatrolTime       time.Time  `json:"patrol_time" gorm:"not null"`
	PatrolArea       *string    `json:"patrol_area" gorm:"type:varchar(200)"`
	Latitude         *float64   `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude        *float64   `json:"longitude" gorm:"type:decimal(11,8)"`
	Photo            *string    `json:"photo" gorm:"type:varchar(255)"`
	Notes            *string    `json:"notes" gorm:"type:text"`
	ValidationStatus string     `json:"validation_status" gorm:"type:varchar(20);default:'pending'"`
	ValidatorID      *uuid.UUID `json:"validator_id" gorm:"type:uuid"`
	ValidatedAt      *time.Time `json:"validated_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	Personnel  *Personnel  `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Location   *Location   `json:"location,omitempty" gorm:"foreignKey:LocationID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Attendance *Attendance `json:"attendance,omitempty" gorm:"foreignKey:AttendanceID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	Validator  *User       `json:"validator,omitempty" gorm:"foreignKey:ValidatorID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (p *Patrol) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type CreatePatrolRequest struct {
	PersonnelID  uuid.UUID  `json:"personnel_id" validate:"required"`
	LocationID   uuid.UUID  `json:"location_id" validate:"required"`
	AttendanceID *uuid.UUID `json:"attendance_id"`
	Date         string     `json:"date" validate:"required"`        // format: 2006-01-02
	PatrolTime   string     `json:"patrol_time" validate:"required"` // format: 2006-01-02T15:04:05Z07:00
	PatrolArea   *string    `json:"patrol_area"`
	Latitude     *float64   `json:"latitude"`
	Longitude    *float64   `json:"longitude"`
	Photo        *string    `json:"photo"`
	Notes        *string    `json:"notes"`
}

type UpdatePatrolRequest struct {
	PatrolArea *string  `json:"patrol_area"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
	Photo      *string  `json:"photo"`
	Notes      *string  `json:"notes"`
}

type ValidatePatrolRequest struct {
	ValidationStatus string  `json:"validation_status" validate:"required,oneof=approved rejected"`
	ValidatorNotes   *string `json:"validator_notes"`
}

type PatrolResponse struct {
	ID               uuid.UUID           `json:"id"`
	PersonnelID      uuid.UUID           `json:"personnel_id"`
	LocationID       uuid.UUID           `json:"location_id"`
	AttendanceID     *uuid.UUID          `json:"attendance_id,omitempty"`
	Date             string              `json:"date"`
	PatrolTime       string              `json:"patrol_time"`
	PatrolArea       *string             `json:"patrol_area,omitempty"`
	Latitude         *float64            `json:"latitude,omitempty"`
	Longitude        *float64            `json:"longitude,omitempty"`
	Photo            *string             `json:"photo,omitempty"`
	PhotoURL         *string             `json:"photo_url,omitempty"`
	Notes            *string             `json:"notes,omitempty"`
	ValidationStatus string              `json:"validation_status"`
	ValidatorID      *uuid.UUID          `json:"validator_id,omitempty"`
	ValidatedAt      *time.Time          `json:"validated_at,omitempty"`
	Personnel        *PersonnelResponse  `json:"personnel,omitempty"`
	Location         *LocationResponse   `json:"location,omitempty"`
	Attendance       *AttendanceResponse `json:"attendance,omitempty"`
	Validator        *UserResponse       `json:"validator,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}
