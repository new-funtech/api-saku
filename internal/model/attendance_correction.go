package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AttendanceCorrection represents requests to correct attendance data
type AttendanceCorrection struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID        uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null"`
	CorrectionDate     time.Time  `json:"correction_date" gorm:"type:date;not null"`
	CorrectionType     string     `json:"correction_type" gorm:"type:varchar(20);not null"` // checkin, checkout, both
	CheckinTime        *string    `json:"checkin_time" gorm:"type:time"`
	CheckoutTime       *string    `json:"checkout_time" gorm:"type:time"`
	Reason             string     `json:"reason" gorm:"type:text;not null"`
	SupportingDocument *string    `json:"supporting_document" gorm:"type:varchar(255)"`
	Status             string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	ApproverID         *uuid.UUID `json:"approver_id" gorm:"type:uuid"`
	ApprovedAt         *time.Time `json:"approved_at"`
	ApproverNotes      *string    `json:"approver_notes" gorm:"type:text"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Relations
	Personnel *Personnel `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Approver  *User      `json:"approver,omitempty" gorm:"foreignKey:ApproverID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (ac *AttendanceCorrection) BeforeCreate(tx *gorm.DB) error {
	if ac.ID == uuid.Nil {
		ac.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreateAttendanceCorrectionRequest struct {
	PersonnelID        uuid.UUID `json:"personnel_id" validate:"required"`
	CorrectionDate     string    `json:"correction_date" validate:"required"` // format: 2006-01-02
	CorrectionType     string    `json:"correction_type" validate:"required,oneof=checkin checkout both"`
	CheckinTime        *string   `json:"checkin_time"`  // format: 15:04:05
	CheckoutTime       *string   `json:"checkout_time"` // format: 15:04:05
	Reason             string    `json:"reason" validate:"required"`
	SupportingDocument *string   `json:"supporting_document"`
}

type UpdateAttendanceCorrectionRequest struct {
	CorrectionDate     *string `json:"correction_date"` // format: 2006-01-02
	CorrectionType     *string `json:"correction_type" validate:"omitempty,oneof=checkin checkout both"`
	CheckinTime        *string `json:"checkin_time"`  // format: 15:04:05
	CheckoutTime       *string `json:"checkout_time"` // format: 15:04:05
	Reason             *string `json:"reason"`
	SupportingDocument *string `json:"supporting_document"`
}

type ApproveAttendanceCorrectionRequest struct {
	Status        string  `json:"status" validate:"required,oneof=approved rejected"`
	ApproverNotes *string `json:"approver_notes"`
}

type AttendanceCorrectionResponse struct {
	ID                    uuid.UUID          `json:"id"`
	PersonnelID           uuid.UUID          `json:"personnel_id"`
	CorrectionDate        string             `json:"correction_date"`
	CorrectionType        string             `json:"correction_type"`
	CheckinTime           *string            `json:"checkin_time,omitempty"`
	CheckoutTime          *string            `json:"checkout_time,omitempty"`
	Reason                string             `json:"reason"`
	SupportingDocument    *string            `json:"supporting_document,omitempty"`
	SupportingDocumentURL *string            `json:"supporting_document_url,omitempty"`
	Status                string             `json:"status"`
	ApproverID            *uuid.UUID         `json:"approver_id,omitempty"`
	ApprovedAt            *time.Time         `json:"approved_at,omitempty"`
	ApproverNotes         *string            `json:"approver_notes,omitempty"`
	Personnel             *PersonnelResponse `json:"personnel,omitempty"`
	Approver              *UserResponse      `json:"approver,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}
