package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Assignment represents personnel work assignment to location
type Assignment struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null"`
	LocationID  uuid.UUID  `json:"location_id" gorm:"type:uuid;not null"`
	ShiftID     *uuid.UUID `json:"shift_id" gorm:"type:uuid"`
	StartDate   time.Time  `json:"start_date" gorm:"type:date;not null"`
	EndDate     *time.Time `json:"end_date" gorm:"type:date"`
	Status      string     `json:"status" gorm:"type:varchar(20);default:'active'"`
	Notes       *string    `json:"notes" gorm:"type:text"`
	CreatedBy   *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Relations
	Personnel *Personnel `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Location  *Location  `json:"location,omitempty" gorm:"foreignKey:LocationID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Shift     *Shift     `json:"shift,omitempty" gorm:"foreignKey:ShiftID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	Creator   *User      `json:"creator,omitempty" gorm:"foreignKey:CreatedBy;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (a *Assignment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreateAssignmentRequest struct {
	PersonnelID uuid.UUID  `json:"personnel_id" validate:"required"`
	LocationID  uuid.UUID  `json:"location_id" validate:"required"`
	ShiftID     *uuid.UUID `json:"shift_id"`
	StartDate   string     `json:"start_date" validate:"required"` // format: 2006-01-02
	EndDate     *string    `json:"end_date"`                       // format: 2006-01-02
	Status      string     `json:"status" validate:"omitempty,oneof=active completed cancelled"`
	Notes       *string    `json:"notes"`
	CreatedBy   *uuid.UUID `json:"created_by"`
}

type UpdateAssignmentRequest struct {
	LocationID *uuid.UUID `json:"location_id"`
	ShiftID    *uuid.UUID `json:"shift_id"`
	StartDate  *string    `json:"start_date"` // format: 2006-01-02
	EndDate    *string    `json:"end_date"`   // format: 2006-01-02
	Status     *string    `json:"status" validate:"omitempty,oneof=active completed cancelled"`
	Notes      *string    `json:"notes"`
}

type AssignmentResponse struct {
	ID          uuid.UUID          `json:"id"`
	PersonnelID uuid.UUID          `json:"personnel_id"`
	LocationID  uuid.UUID          `json:"location_id"`
	ShiftID     *uuid.UUID         `json:"shift_id,omitempty"`
	StartDate   string             `json:"start_date"`
	EndDate     *string            `json:"end_date,omitempty"`
	Status      string             `json:"status"`
	Notes       *string            `json:"notes,omitempty"`
	CreatedBy   *uuid.UUID         `json:"created_by,omitempty"`
	Personnel   *PersonnelResponse `json:"personnel,omitempty"`
	Location    *LocationResponse  `json:"location,omitempty"`
	Shift       *ShiftResponse     `json:"shift,omitempty"`
	Creator     *UserResponse      `json:"creator,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}
