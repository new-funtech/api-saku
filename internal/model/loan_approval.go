package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Loan approval levels
const (
	LoanApprovalLevelBujp  = 1
	LoanApprovalLevelPusat = 2

	LoanApprovalStatusPending  = "pending"
	LoanApprovalStatusApproved = "approved"
	LoanApprovalStatusRejected = "rejected"
)

type LoanApproval struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	LoanID            uuid.UUID  `json:"loan_id" gorm:"type:uuid;not null;index"`
	ApprovalLevel     int        `json:"approval_level" gorm:"not null"`
	ApprovalLevelName string     `json:"approval_level_name" gorm:"type:varchar(50);not null"`
	ApproverID        *uuid.UUID `json:"approver_id" gorm:"type:uuid"`
	Status            string     `json:"status" gorm:"type:varchar(20);not null;default:'pending';index"`
	Notes             *string    `json:"notes" gorm:"type:text"`
	ApprovedAmount    *float64   `json:"approved_amount" gorm:"type:decimal(14,2)"`
	ApprovedTenor     *int       `json:"approved_tenor"`
	ReviewedAt        *time.Time `json:"reviewed_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	Loan     *Loan `json:"loan,omitempty" gorm:"foreignKey:LoanID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Approver *User `json:"approver,omitempty" gorm:"foreignKey:ApproverID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (la *LoanApproval) BeforeCreate(tx *gorm.DB) error {
	if la.ID == uuid.Nil {
		la.ID = uuid.New()
	}
	return nil
}

type LoanApprovalRequest struct {
	Action         string   `json:"action" validate:"required,oneof=approve reject"`
	Notes          *string  `json:"notes"`
	ApprovedAmount *float64 `json:"approved_amount" validate:"omitempty,gt=0"`
	ApprovedTenor  *int     `json:"approved_tenor" validate:"omitempty,gte=1,lte=120"`
}

type LoanApprovalResponse struct {
	ID                uuid.UUID     `json:"id"`
	LoanID            uuid.UUID     `json:"loan_id"`
	ApprovalLevel     int           `json:"approval_level"`
	ApprovalLevelName string        `json:"approval_level_name"`
	ApproverID        *uuid.UUID    `json:"approver_id,omitempty"`
	Status            string        `json:"status"`
	Notes             *string       `json:"notes,omitempty"`
	ApprovedAmount    *float64      `json:"approved_amount,omitempty"`
	ApprovedTenor     *int          `json:"approved_tenor,omitempty"`
	ReviewedAt        *time.Time    `json:"reviewed_at,omitempty"`
	Approver          *UserResponse `json:"approver,omitempty"`
	Loan              *LoanResponse `json:"loan,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}
