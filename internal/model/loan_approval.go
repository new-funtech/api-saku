package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LoanApprovalLevelBujp  = 1
	LoanApprovalLevelPusat = 2
	LoanApprovalLevelBprks = 3

	LoanApprovalStatusPending  = "pending"
	LoanApprovalStatusApproved = "approved"
	LoanApprovalStatusRejected = "rejected"
)

const (
	SanctionStatusNeverSanctioned      = "never_sanctioned"
	SanctionStatusPreviouslySanctioned = "previously_sanctioned"
	SanctionStatusCurrentlySanctioned  = "currently_sanctioned"
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
	// Employee disciplinary status as judged by the Admin Perusahaan (level
	// 1) reviewer during verification — there is no automated
	// disciplinary/sanction record system in this application, so this is a
	// manual attestation recorded at approval time, not derived data.
	SanctionStatus *string    `json:"sanction_status" gorm:"type:varchar(30)"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

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
	// Recorded by the Admin Perusahaan reviewer only — see LoanApproval.SanctionStatus.
	SanctionStatus *string `json:"sanction_status" validate:"omitempty,oneof=never_sanctioned previously_sanctioned currently_sanctioned"`
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
	SanctionStatus    *string       `json:"sanction_status,omitempty"`
	ReviewedAt        *time.Time    `json:"reviewed_at,omitempty"`
	Approver          *UserResponse `json:"approver,omitempty"`
	Loan              *LoanResponse `json:"loan,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}
