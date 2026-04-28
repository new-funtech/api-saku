package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Attendance represents daily attendance records
type Attendance struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID        uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null;uniqueIndex:idx_personnel_date"`
	LocationID         uuid.UUID  `json:"location_id" gorm:"type:uuid;not null"`
	AssignmentID       *uuid.UUID `json:"assignment_id" gorm:"type:uuid"`
	Date               time.Time  `json:"date" gorm:"type:date;not null;uniqueIndex:idx_personnel_date"`
	CheckIn            *time.Time `json:"check_in"`
	CheckOut           *time.Time `json:"check_out"`
	CheckInLatitude    *string    `json:"check_in_latitude" gorm:"type:decimal(10,8)"`
	CheckInLongitude   *string    `json:"check_in_longitude" gorm:"type:decimal(11,8)"`
	CheckOutLatitude   *string    `json:"check_out_latitude" gorm:"type:decimal(10,8)"`
	CheckOutLongitude  *string    `json:"check_out_longitude" gorm:"type:decimal(11,8)"`
	CheckInPhoto       *string    `json:"check_in_photo" gorm:"type:text"`
	CheckOutPhoto      *string    `json:"check_out_photo" gorm:"type:text"`
	SupportingDocument *string    `json:"supporting_document" gorm:"type:text"`
	Status             string     `json:"status" gorm:"type:varchar(20);default:'present'"`
	Notes              *string    `json:"notes" gorm:"type:text"`
	WorkDuration       *int       `json:"work_duration"` // in minutes
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Relations
	Personnel  *Personnel  `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Location   *Location   `json:"location,omitempty" gorm:"foreignKey:LocationID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Assignment *Assignment `json:"assignment,omitempty" gorm:"foreignKey:AssignmentID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (a *Attendance) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreateAttendanceRequest struct {
	PersonnelID        uuid.UUID    `json:"personnel_id" validate:"required"`
	LocationID         uuid.UUID    `json:"location_id" validate:"required"`
	AssignmentID       *uuid.UUID   `json:"assignment_id"`
	Date               string       `json:"date" validate:"required"` // format: 2006-01-02
	CheckIn            *string      `json:"check_in"`                 // format: 2006-01-02T15:04:05Z07:00
	CheckOut           *string      `json:"check_out"`
	CheckInLatitude    NumberString `json:"check_in_latitude"`
	CheckInLongitude   NumberString `json:"check_in_longitude"`
	CheckOutLatitude   NumberString `json:"check_out_latitude"`
	CheckOutLongitude  NumberString `json:"check_out_longitude"`
	CheckInPhoto       *string      `json:"check_in_photo"`
	CheckOutPhoto      *string      `json:"check_out_photo"`
	SupportingDocument *string      `json:"supporting_document"`
	Status             string       `json:"status" validate:"omitempty,oneof=present late permission sick absent leave"`
	Notes              *string      `json:"notes"`
}

type UpdateAttendanceRequest struct {
	CheckOut           *string      `json:"check_out"`
	CheckOutLatitude   NumberString `json:"check_out_latitude"`
	CheckOutLongitude  NumberString `json:"check_out_longitude"`
	CheckOutPhoto      *string      `json:"check_out_photo"`
	SupportingDocument *string      `json:"supporting_document"`
	Status             *string      `json:"status" validate:"omitempty,oneof=present late permission sick absent leave"`
	Notes              *string      `json:"notes"`
	WorkDuration       *int         `json:"work_duration"`
}

type AttendanceResponse struct {
	ID                    uuid.UUID           `json:"id"`
	PersonnelID           uuid.UUID           `json:"personnel_id"`
	LocationID            uuid.UUID           `json:"location_id"`
	AssignmentID          *uuid.UUID          `json:"assignment_id,omitempty"`
	Date                  string              `json:"date"`
	CheckIn               *string             `json:"check_in,omitempty"`
	CheckOut              *string             `json:"check_out,omitempty"`
	CheckInLatitude       *string             `json:"check_in_latitude,omitempty"`
	CheckInLongitude      *string             `json:"check_in_longitude,omitempty"`
	CheckOutLatitude      *string             `json:"check_out_latitude,omitempty"`
	CheckOutLongitude     *string             `json:"check_out_longitude,omitempty"`
	CheckInPhoto          *string             `json:"check_in_photo,omitempty"`
	CheckInPhotoURL       *string             `json:"check_in_photo_url,omitempty"`
	CheckOutPhoto         *string             `json:"check_out_photo,omitempty"`
	CheckOutPhotoURL      *string             `json:"check_out_photo_url,omitempty"`
	SupportingDocument    *string             `json:"supporting_document,omitempty"`
	SupportingDocumentURL *string             `json:"supporting_document_url,omitempty"`
	Status                string              `json:"status"`
	Notes                 *string             `json:"notes,omitempty"`
	WorkDuration          *int                `json:"work_duration,omitempty"`
	Personnel             *PersonnelResponse  `json:"personnel,omitempty"`
	Location              *LocationResponse   `json:"location,omitempty"`
	Assignment            *AssignmentResponse `json:"assignment,omitempty"`
	CreatedAt             time.Time           `json:"created_at"`
	UpdatedAt             time.Time           `json:"updated_at"`
}
