package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Overtime is a request for extra working hours on one day, decided by an admin.
type Overtime struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID   uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null;index"`
	Date          time.Time  `json:"date" gorm:"type:date;not null;index"`
	StartTime     string     `json:"start_time" gorm:"type:varchar(5);not null"` // HH:MM
	EndTime       string     `json:"end_time" gorm:"type:varchar(5);not null"`   // HH:MM
	TotalHours    float64    `json:"total_hours" gorm:"type:decimal(4,2);not null"`
	Reason        string     `json:"reason" gorm:"type:text;not null"`
	Status        string     `json:"status" gorm:"type:varchar(20);not null;default:'pending';index"`
	ApproverID    *uuid.UUID `json:"approver_id" gorm:"type:uuid"`
	ApprovedAt    *time.Time `json:"approved_at"`
	ApproverNotes *string    `json:"approver_notes" gorm:"type:text"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	Personnel *Personnel `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Approver  *User      `json:"approver,omitempty" gorm:"foreignKey:ApproverID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (o *Overtime) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

type CreateOvertimeRequest struct {
	// PersonnelID is ignored for staff: they can only request for themselves.
	PersonnelID uuid.UUID `json:"personnel_id"`
	Date        string    `json:"date" validate:"required"`       // 2006-01-02
	StartTime   string    `json:"start_time" validate:"required"` // HH:MM
	EndTime     string    `json:"end_time" validate:"required"`   // HH:MM
	Reason      string    `json:"reason" validate:"required"`
}

type UpdateOvertimeRequest struct {
	Date      *string `json:"date"`
	StartTime *string `json:"start_time"`
	EndTime   *string `json:"end_time"`
	Reason    *string `json:"reason"`
}

type DecideOvertimeRequest struct {
	Status        string  `json:"status"`
	ApproverNotes *string `json:"approver_notes"`
}

type OvertimeResponse struct {
	ID            uuid.UUID          `json:"id"`
	PersonnelID   uuid.UUID          `json:"personnel_id"`
	Date          string             `json:"date"`
	StartTime     string             `json:"start_time"`
	EndTime       string             `json:"end_time"`
	TotalHours    float64            `json:"total_hours"`
	Reason        string             `json:"reason"`
	Status        string             `json:"status"`
	ApproverID    *uuid.UUID         `json:"approver_id,omitempty"`
	ApprovedAt    *time.Time         `json:"approved_at,omitempty"`
	ApproverNotes *string            `json:"approver_notes,omitempty"`
	Personnel     *PersonnelResponse `json:"personnel,omitempty"`
	Approver      *UserResponse      `json:"approver,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}
