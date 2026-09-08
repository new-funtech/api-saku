package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Leave struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID        uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null"`
	Type               string     `json:"type" gorm:"type:varchar(20);not null"` // leave, permission, sick
	StartDate          time.Time  `json:"start_date" gorm:"type:date;not null"`
	EndDate            time.Time  `json:"end_date" gorm:"type:date;not null"`
	TotalDays          *int       `json:"total_days"`
	Reason             string     `json:"reason" gorm:"type:text;not null"`
	SupportingDocument *string    `json:"supporting_document" gorm:"type:varchar(255)"`
	Status             string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	ApproverID         *uuid.UUID `json:"approver_id" gorm:"type:uuid"`
	ApprovedAt         *time.Time `json:"approved_at"`
	ApproverNotes      *string    `json:"approver_notes" gorm:"type:text"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	Personnel *Personnel `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Approver  *User      `json:"approver,omitempty" gorm:"foreignKey:ApproverID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (l *Leave) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type CreateLeaveRequest struct {
	PersonnelID        uuid.UUID `json:"personnel_id" validate:"required"`
	Type               string    `json:"type" validate:"required,oneof=leave permission sick"`
	StartDate          string    `json:"start_date" validate:"required"` // format: 2006-01-02
	EndDate            string    `json:"end_date" validate:"required"`   // format: 2006-01-02
	TotalDays          *int      `json:"total_days"`
	Reason             string    `json:"reason" validate:"required"`
	SupportingDocument *string   `json:"supporting_document"`
}

type UpdateLeaveRequest struct {
	Type               *string `json:"type" validate:"omitempty,oneof=leave permission sick"`
	StartDate          *string `json:"start_date"` // format: 2006-01-02
	EndDate            *string `json:"end_date"`   // format: 2006-01-02
	TotalDays          *int    `json:"total_days"`
	Reason             *string `json:"reason"`
	SupportingDocument *string `json:"supporting_document"`
}

type ApproveLeaveRequest struct {
	Status        string  `json:"status" validate:"required,oneof=approved rejected"`
	ApproverNotes *string `json:"approver_notes"`
}

type LeaveResponse struct {
	ID                    uuid.UUID          `json:"id"`
	PersonnelID           uuid.UUID          `json:"personnel_id"`
	Type                  string             `json:"type"`
	StartDate             string             `json:"start_date"`
	EndDate               string             `json:"end_date"`
	TotalDays             *int               `json:"total_days,omitempty"`
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
