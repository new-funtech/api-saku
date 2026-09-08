package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PayrollDetail struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	PayrollID         uuid.UUID `json:"payroll_id" gorm:"type:uuid;not null"`
	SalaryComponentID uuid.UUID `json:"salary_component_id" gorm:"type:uuid;not null"`
	Amount            float64   `json:"amount" gorm:"type:decimal(12,2);not null"`
	Notes             *string   `json:"notes" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`

	// Relations
	Payroll         *Payroll         `json:"payroll,omitempty" gorm:"foreignKey:PayrollID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	SalaryComponent *SalaryComponent `json:"salary_component,omitempty" gorm:"foreignKey:SalaryComponentID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
}

func (pd *PayrollDetail) BeforeCreate(tx *gorm.DB) error {
	if pd.ID == uuid.Nil {
		pd.ID = uuid.New()
	}
	return nil
}

type CreatePayrollDetailRequest struct {
	SalaryComponentID uuid.UUID `json:"salary_component_id" validate:"required"`
	Amount            float64   `json:"amount" validate:"required"`
	Notes             *string   `json:"notes"`
}

type PayrollDetailResponse struct {
	ID                uuid.UUID                `json:"id"`
	PayrollID         uuid.UUID                `json:"payroll_id"`
	SalaryComponentID uuid.UUID                `json:"salary_component_id"`
	Amount            float64                  `json:"amount"`
	Notes             *string                  `json:"notes,omitempty"`
	SalaryComponent   *SalaryComponentResponse `json:"salary_component,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
}
