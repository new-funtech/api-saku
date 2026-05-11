package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MonthlyReport struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID          uuid.UUID  `json:"bujp_id" gorm:"type:uuid;not null"`
	Period          string     `json:"period" gorm:"type:varchar(7);not null"` // YYYY-MM
	TotalGuards     int        `json:"total_guards" gorm:"default:0"`
	TotalLocations  int        `json:"total_locations" gorm:"default:0"`
	TotalPresent    int        `json:"total_present" gorm:"default:0"`
	TotalPermission int        `json:"total_permission" gorm:"default:0"`
	TotalSick       int        `json:"total_sick" gorm:"default:0"`
	TotalAbsent     int        `json:"total_absent" gorm:"default:0"`
	TotalPatrols    int        `json:"total_patrols" gorm:"default:0"`
	TotalPayroll    float64    `json:"total_payroll" gorm:"type:decimal(15,2);default:0"`
	ReportFile      *string    `json:"report_file" gorm:"type:varchar(255)"`
	CreatedBy       *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	Bujp    *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Creator *User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (mr *MonthlyReport) BeforeCreate(tx *gorm.DB) error {
	if mr.ID == uuid.Nil {
		mr.ID = uuid.New()
	}
	return nil
}

type CreateMonthlyReportRequest struct {
	BujpID          uuid.UUID `json:"bujp_id" validate:"required"`
	Period          string    `json:"period" validate:"required"` // YYYY-MM
	TotalGuards     int       `json:"total_guards" validate:"gte=0"`
	TotalLocations  int       `json:"total_locations" validate:"gte=0"`
	TotalPresent    int       `json:"total_present" validate:"gte=0"`
	TotalPermission int       `json:"total_permission" validate:"gte=0"`
	TotalSick       int       `json:"total_sick" validate:"gte=0"`
	TotalAbsent     int       `json:"total_absent" validate:"gte=0"`
	TotalPatrols    int       `json:"total_patrols" validate:"gte=0"`
	TotalPayroll    float64   `json:"total_payroll" validate:"gte=0"`
	ReportFile      *string   `json:"report_file"`
}

type GenerateMonthlyReportRequest struct {
	BujpID uuid.UUID `json:"bujp_id"`
	Period string    `json:"period" validate:"required"` // YYYY-MM
	Force  bool      `json:"force"`
}

type UpdateMonthlyReportRequest struct {
	Period          *string  `json:"period"`
	TotalGuards     *int     `json:"total_guards" validate:"omitempty,gte=0"`
	TotalLocations  *int     `json:"total_locations" validate:"omitempty,gte=0"`
	TotalPresent    *int     `json:"total_present" validate:"omitempty,gte=0"`
	TotalPermission *int     `json:"total_permission" validate:"omitempty,gte=0"`
	TotalSick       *int     `json:"total_sick" validate:"omitempty,gte=0"`
	TotalAbsent     *int     `json:"total_absent" validate:"omitempty,gte=0"`
	TotalPatrols    *int     `json:"total_patrols" validate:"omitempty,gte=0"`
	TotalPayroll    *float64 `json:"total_payroll" validate:"omitempty,gte=0"`
	ReportFile      *string  `json:"report_file"`
}

type MonthlyReportResponse struct {
	ID              uuid.UUID     `json:"id"`
	BujpID          uuid.UUID     `json:"bujp_id"`
	Period          string        `json:"period"`
	TotalGuards     int           `json:"total_guards"`
	TotalLocations  int           `json:"total_locations"`
	TotalPresent    int           `json:"total_present"`
	TotalPermission int           `json:"total_permission"`
	TotalSick       int           `json:"total_sick"`
	TotalAbsent     int           `json:"total_absent"`
	TotalPatrols    int           `json:"total_patrols"`
	TotalPayroll    float64       `json:"total_payroll"`
	ReportFile      *string       `json:"report_file,omitempty"`
	CreatedBy       *uuid.UUID    `json:"created_by,omitempty"`
	Bujp            *BujpResponse `json:"bujp,omitempty"`
	Creator         *UserResponse `json:"creator,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
