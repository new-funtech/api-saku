package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LoanInstallmentStatusPending = "pending"
	LoanInstallmentStatusPaid    = "paid"
	LoanInstallmentStatusOverdue = "overdue"
	LoanInstallmentStatusWaived  = "waived"
)

type LoanInstallment struct {
	ID                   uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	LoanID               uuid.UUID  `json:"loan_id" gorm:"type:uuid;not null;index"`
	InstallmentNumber    int        `json:"installment_number" gorm:"not null"`
	DueDate              time.Time  `json:"due_date" gorm:"type:date;not null;index"`
	PrincipalAmount      float64    `json:"principal_amount" gorm:"type:decimal(14,2);not null"`
	InterestAmount       float64    `json:"interest_amount" gorm:"type:decimal(14,2);not null"`
	InstallmentAmount    float64    `json:"installment_amount" gorm:"type:decimal(14,2);not null"`
	Status               string     `json:"status" gorm:"type:varchar(20);not null;default:'pending';index"`
	PaidAmount           float64    `json:"paid_amount" gorm:"type:decimal(14,2);default:0"`
	RemainingAmount      float64    `json:"remaining_amount" gorm:"type:decimal(14,2);default:0"`
	PaymentDate          *time.Time `json:"payment_date" gorm:"type:date"`
	PaymentMethod        *string    `json:"payment_method" gorm:"type:varchar(30)"`
	PayrollID            *uuid.UUID `json:"payroll_id" gorm:"type:uuid"`
	TransactionReference *string    `json:"transaction_reference" gorm:"type:varchar(100)"`
	LateDays             int        `json:"late_days" gorm:"default:0"`
	LateFee              float64    `json:"late_fee" gorm:"type:decimal(14,2);default:0"`
	Notes                *string    `json:"notes" gorm:"type:text"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	Loan    *Loan    `json:"loan,omitempty" gorm:"foreignKey:LoanID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Payroll *Payroll `json:"payroll,omitempty" gorm:"foreignKey:PayrollID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (li *LoanInstallment) BeforeCreate(tx *gorm.DB) error {
	if li.ID == uuid.Nil {
		li.ID = uuid.New()
	}
	return nil
}

type LoanInstallmentPaymentRequest struct {
	PaidAmount           float64 `json:"paid_amount"`
	PaymentDate          string  `json:"payment_date"` // YYYY-MM-DD, optional
	PaymentMethod        *string `json:"payment_method"`
	TransactionReference *string `json:"transaction_reference"`
	Notes                *string `json:"notes"`
}

type LoanInstallmentResponse struct {
	ID                   uuid.UUID  `json:"id"`
	LoanID               uuid.UUID  `json:"loan_id"`
	InstallmentNumber    int        `json:"installment_number"`
	DueDate              string     `json:"due_date"`
	PrincipalAmount      float64    `json:"principal_amount"`
	InterestAmount       float64    `json:"interest_amount"`
	InstallmentAmount    float64    `json:"installment_amount"`
	Status               string     `json:"status"`
	PaidAmount           float64    `json:"paid_amount"`
	RemainingAmount      float64    `json:"remaining_amount"`
	PaymentDate          *string    `json:"payment_date,omitempty"`
	PaymentMethod        *string    `json:"payment_method,omitempty"`
	PayrollID            *uuid.UUID `json:"payroll_id,omitempty"`
	TransactionReference *string    `json:"transaction_reference,omitempty"`
	LateDays             int        `json:"late_days"`
	LateFee              float64    `json:"late_fee"`
	Notes                *string    `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
