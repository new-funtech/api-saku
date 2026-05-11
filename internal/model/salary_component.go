package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SalaryComponent struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID            *uuid.UUID `json:"bujp_id" gorm:"type:uuid"`
	Code              string     `json:"code" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name              string     `json:"name" gorm:"type:varchar(100);not null"`
	Type              string     `json:"type" gorm:"type:varchar(20);not null"`               // allowance, deduction
	CalculationMethod string     `json:"calculation_method" gorm:"type:varchar(20);not null"` // fixed, percentage
	Amount            float64    `json:"amount" gorm:"type:decimal(12,2);default:0"`
	IsActive          bool       `json:"is_active" gorm:"default:true"`
	Description       *string    `json:"description" gorm:"type:text"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	Bujp *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (s *SalaryComponent) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type CreateSalaryComponentRequest struct {
	BujpID            *uuid.UUID `json:"bujp_id"`
	Code              string     `json:"code" validate:"required,max=20"`
	Name              string     `json:"name" validate:"required,max=100"`
	Type              string     `json:"type" validate:"required,oneof=allowance deduction"`
	CalculationMethod string     `json:"calculation_method" validate:"required,oneof=fixed percentage"`
	Amount            float64    `json:"amount" validate:"gte=0"`
	IsActive          *bool      `json:"is_active"`
	Description       *string    `json:"description"`
}

type UpdateSalaryComponentRequest struct {
	BujpID            *uuid.UUID `json:"bujp_id"`
	Code              *string    `json:"code" validate:"omitempty,max=20"`
	Name              *string    `json:"name" validate:"omitempty,max=100"`
	Type              *string    `json:"type" validate:"omitempty,oneof=allowance deduction"`
	CalculationMethod *string    `json:"calculation_method" validate:"omitempty,oneof=fixed percentage"`
	Amount            *float64   `json:"amount" validate:"omitempty,gte=0"`
	IsActive          *bool      `json:"is_active"`
	Description       *string    `json:"description"`
}

type SalaryComponentResponse struct {
	ID                uuid.UUID     `json:"id"`
	BujpID            *uuid.UUID    `json:"bujp_id,omitempty"`
	Code              string        `json:"code"`
	Name              string        `json:"name"`
	Type              string        `json:"type"`
	CalculationMethod string        `json:"calculation_method"`
	Amount            float64       `json:"amount"`
	IsActive          bool          `json:"is_active"`
	Description       *string       `json:"description,omitempty"`
	Bujp              *BujpResponse `json:"bujp,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}
