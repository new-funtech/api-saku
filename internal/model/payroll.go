package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Payroll represents monthly payroll/slip gaji
type Payroll struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	PersonnelID     uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null"`
	BujpID          uuid.UUID  `json:"bujp_id" gorm:"type:uuid;not null"`
	Period          string     `json:"period" gorm:"type:varchar(7);not null"` // YYYY-MM
	BaseSalary      float64    `json:"base_salary" gorm:"type:decimal(12,2);not null"`
	TotalAllowances float64    `json:"total_allowances" gorm:"type:decimal(12,2);default:0"`
	TotalDeductions float64    `json:"total_deductions" gorm:"type:decimal(12,2);default:0"`
	NetSalary       float64    `json:"net_salary" gorm:"type:decimal(12,2);not null"`
	TotalPresent    int        `json:"total_present" gorm:"default:0"`
	TotalPermission int        `json:"total_permission" gorm:"default:0"`
	TotalAbsent     int        `json:"total_absent" gorm:"default:0"`
	PaymentStatus   string     `json:"payment_status" gorm:"type:varchar(20);default:'pending'"`
	PaymentDate     *time.Time `json:"payment_date" gorm:"type:date"`
	PaymentMethod   *string    `json:"payment_method" gorm:"type:varchar(30)"`
	CreatedBy       *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Relations
	Personnel      *Personnel      `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Bujp           *Bujp           `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Creator        *User           `json:"creator,omitempty" gorm:"foreignKey:CreatedBy;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	PayrollDetails []PayrollDetail `json:"payroll_details,omitempty" gorm:"foreignKey:PayrollID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
}

func (p *Payroll) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreatePayrollRequest struct {
	PersonnelID     uuid.UUID                    `json:"personnel_id" validate:"required"`
	BujpID          uuid.UUID                    `json:"bujp_id" validate:"required"`
	Period          string                       `json:"period" validate:"required"` // YYYY-MM
	BaseSalary      float64                      `json:"base_salary" validate:"required,gte=0"`
	TotalPresent    int                          `json:"total_present" validate:"gte=0"`
	TotalPermission int                          `json:"total_permission" validate:"gte=0"`
	TotalAbsent     int                          `json:"total_absent" validate:"gte=0"`
	PaymentStatus   *string                      `json:"payment_status" validate:"omitempty,oneof=pending paid cancelled"`
	PaymentDate     *string                      `json:"payment_date"` // YYYY-MM-DD
	PaymentMethod   *string                      `json:"payment_method"`
	Details         []CreatePayrollDetailRequest `json:"details"`
}

type UpdatePayrollRequest struct {
	Period          *string  `json:"period"`
	BaseSalary      *float64 `json:"base_salary" validate:"omitempty,gte=0"`
	TotalPresent    *int     `json:"total_present" validate:"omitempty,gte=0"`
	TotalPermission *int     `json:"total_permission" validate:"omitempty,gte=0"`
	TotalAbsent     *int     `json:"total_absent" validate:"omitempty,gte=0"`
	PaymentStatus   *string  `json:"payment_status" validate:"omitempty,oneof=pending paid cancelled"`
	PaymentDate     *string  `json:"payment_date"` // YYYY-MM-DD
	PaymentMethod   *string  `json:"payment_method"`
}

type PayrollResponse struct {
	ID              uuid.UUID               `json:"id"`
	PersonnelID     uuid.UUID               `json:"personnel_id"`
	BujpID          uuid.UUID               `json:"bujp_id"`
	Period          string                  `json:"period"`
	BaseSalary      float64                 `json:"base_salary"`
	TotalAllowances float64                 `json:"total_allowances"`
	TotalDeductions float64                 `json:"total_deductions"`
	NetSalary       float64                 `json:"net_salary"`
	TotalPresent    int                     `json:"total_present"`
	TotalPermission int                     `json:"total_permission"`
	TotalAbsent     int                     `json:"total_absent"`
	PaymentStatus   string                  `json:"payment_status"`
	PaymentDate     *time.Time              `json:"payment_date,omitempty"`
	PaymentMethod   *string                 `json:"payment_method,omitempty"`
	CreatedBy       *uuid.UUID              `json:"created_by,omitempty"`
	Personnel       *PersonnelResponse      `json:"personnel,omitempty"`
	Bujp            *BujpResponse           `json:"bujp,omitempty"`
	Creator         *UserResponse           `json:"creator,omitempty"`
	PayrollDetails  []PayrollDetailResponse `json:"payroll_details,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// GeneratePayrollRequest mirrors laravel-backend PayrollController@generate.
type GeneratePayrollRequest struct {
	Period string    `json:"period" validate:"required"` // YYYY-MM
	BujpID uuid.UUID `json:"bujp_id" validate:"required"`
	Force  bool      `json:"force"`
}

// GeneratePayrollResult summarises which personnel were processed.
type GeneratePayrollResult struct {
	Generated []string          `json:"generated"`
	Skipped   []string          `json:"skipped"`
	Payrolls  []PayrollResponse `json:"payrolls"`
}
