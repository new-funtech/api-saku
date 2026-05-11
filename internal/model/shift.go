package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Shift struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID        *uuid.UUID `json:"bujp_id" gorm:"type:uuid;index"`
	Name          string     `json:"name" gorm:"type:varchar(50);not null;index"`
	Description   *string    `json:"description" gorm:"type:text"`
	StartTime     TimeOnly   `json:"start_time" gorm:"type:time;not null"`
	EndTime       TimeOnly   `json:"end_time" gorm:"type:time;not null"`
	DurationHours *float64   `json:"duration_hours" gorm:"type:decimal(4,2)"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:'active';index"`
	CreatedAt     time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// Relations
	Bujp *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (s *Shift) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type CreateShiftRequest struct {
	BujpID        *uuid.UUID `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          string     `json:"name" validate:"required,max=50" example:"Shift Pagi"`
	Description   *string    `json:"description" example:"Shift pagi dari pukul 07:00 hingga 15:00"`
	StartTime     TimeOnly   `json:"start_time" validate:"required" example:"07:00" swaggertype:"string"`
	EndTime       TimeOnly   `json:"end_time" validate:"required" example:"15:00" swaggertype:"string"`
	DurationHours *float64   `json:"duration_hours" example:"8"`
	Status        string     `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type UpdateShiftRequest struct {
	BujpID        *uuid.UUID `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          *string    `json:"name" validate:"omitempty,max=50" example:"Shift Pagi"`
	Description   *string    `json:"description" example:"Shift pagi dari pukul 07:00 hingga 15:00"`
	StartTime     *TimeOnly  `json:"start_time" example:"07:00" swaggertype:"string"`
	EndTime       *TimeOnly  `json:"end_time" example:"15:00" swaggertype:"string"`
	DurationHours *float64   `json:"duration_hours" example:"8"`
	Status        *string    `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type ShiftResponse struct {
	ID            uuid.UUID     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	BujpID        *uuid.UUID    `json:"bujp_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          string        `json:"name" example:"Shift Pagi"`
	Description   *string       `json:"description,omitempty" example:"Shift pagi dari pukul 07:00 hingga 15:00"`
	StartTime     TimeOnly      `json:"start_time" example:"07:00:00" swaggertype:"string"`
	EndTime       TimeOnly      `json:"end_time" example:"15:00:00" swaggertype:"string"`
	DurationHours *float64      `json:"duration_hours,omitempty" example:"8"`
	Status        string        `json:"status" example:"active"`
	Bujp          *BujpResponse `json:"bujp,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}
