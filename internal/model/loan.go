package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LoanStatusDraft                   = "draft"
	LoanStatusSubmitted               = "submitted"
	LoanStatusApprovedBujp            = "approved_bujp"
	LoanStatusApprovedPusat           = "approved_pusat"
	LoanStatusPendingUserConfirmation = "pending_user_confirmation"
	LoanStatusRejected                = "rejected"
	LoanStatusDisbursed               = "disbursed"
	LoanStatusActive                  = "active"
	LoanStatusCompleted               = "completed"
	LoanStatusCancelled               = "cancelled"
)

type Loan struct {
	ID                       uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	LoanNumber               string     `json:"loan_number" gorm:"type:varchar(30);uniqueIndex;not null"`
	PersonnelID              uuid.UUID  `json:"personnel_id" gorm:"type:uuid;not null;index"`
	BujpID                   *uuid.UUID `json:"bujp_id" gorm:"type:uuid;index"`
	LoanProductID            *uuid.UUID `json:"loan_product_id" gorm:"type:uuid;index"`
	Purpose                  string     `json:"purpose" gorm:"type:text;not null"`
	LoanAmount               float64    `json:"loan_amount" gorm:"type:decimal(14,2);not null"`
	InterestRate             float64    `json:"interest_rate" gorm:"type:decimal(5,2);not null"`
	TenorMonths              int        `json:"tenor_months" gorm:"not null"`
	MonthlyInstallment       float64    `json:"monthly_installment" gorm:"type:decimal(14,2);not null"`
	TotalRepayment           float64    `json:"total_repayment" gorm:"type:decimal(14,2);not null"`
	ApprovedAmount           *float64   `json:"approved_amount" gorm:"type:decimal(14,2)"`
	ApprovedTenor            *int       `json:"approved_tenor"`
	DisbursementDate         *time.Time `json:"disbursement_date" gorm:"type:date"`
	FirstInstallmentDate     *time.Time `json:"first_installment_date" gorm:"type:date"`
	DeductFromPayroll        bool       `json:"deduct_from_payroll" gorm:"default:true"`
	KtpDocument              *string    `json:"ktp_document" gorm:"type:varchar(255)"`
	NpwpDocument             *string    `json:"npwp_document" gorm:"type:varchar(255)"`
	SelfieDocument           *string    `json:"selfie_document" gorm:"type:varchar(255)"`
	SelfieKtpDocument        *string    `json:"selfie_ktp_document" gorm:"type:varchar(255)"`
	PksDocument              *string    `json:"pks_document" gorm:"type:varchar(255)"`
	CollateralDocument       *string    `json:"collateral_document" gorm:"type:varchar(255)"`
	PlacementCompanyID       *uuid.UUID `json:"placement_company_id" gorm:"type:uuid"`
	PlacementCompanyName     *string    `json:"placement_company_name" gorm:"type:varchar(150)"`
	PlacementBujpName        *string    `json:"placement_bujp_name" gorm:"type:varchar(150)"`
	PlacementAddress         *string    `json:"placement_address" gorm:"type:varchar(255)"`
	PlacementDuration        *string    `json:"placement_duration" gorm:"type:varchar(50)"`
	PlacementLocation        *string    `json:"placement_location" gorm:"type:varchar(150)"`
	Status                   string     `json:"status" gorm:"type:varchar(40);not null;default:'draft';index"`
	SubmittedAt              *time.Time `json:"submitted_at"`
	ApprovedAt               *time.Time `json:"approved_at"`
	ApprovedPusatAt          *time.Time `json:"approved_pusat_at"`
	ApprovedBprksAt          *time.Time `json:"approved_bprks_at"`
	RejectedAt               *time.Time `json:"rejected_at"`
	RejectionReason          *string    `json:"rejection_reason" gorm:"type:text"`
	UserConfirmationDeadline *time.Time `json:"user_confirmation_deadline"`
	UserConfirmedAt          *time.Time `json:"user_confirmed_at"`
	CompletedAt              *time.Time `json:"completed_at"`
	CreatedBy                *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`

	Personnel    *Personnel        `json:"personnel,omitempty" gorm:"foreignKey:PersonnelID;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE"`
	Bujp         *Bujp             `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	LoanProduct  *LoanProduct      `json:"loan_product,omitempty" gorm:"foreignKey:LoanProductID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	Creator      *User             `json:"creator,omitempty" gorm:"foreignKey:CreatedBy;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	Approvals    []LoanApproval    `json:"approvals,omitempty" gorm:"foreignKey:LoanID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Installments []LoanInstallment `json:"installments,omitempty" gorm:"foreignKey:LoanID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
}

func (l *Loan) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type CreateLoanRequest struct {
	PersonnelID        *uuid.UUID `json:"personnel_id"` // optional - inferred from auth user if absent
	LoanProductID      *uuid.UUID `json:"loan_product_id"`
	Purpose            string     `json:"purpose" validate:"omitempty,max=500"`
	LoanAmount         float64    `json:"loan_amount" validate:"required,gt=0"`
	InterestRate       *float64   `json:"interest_rate" validate:"omitempty,gte=0,lte=100"`
	TenorMonths        int        `json:"tenor_months" validate:"required,gte=1,lte=120"`
	DeductFromPayroll  *bool      `json:"deduct_from_payroll"`
	KtpDocument        *string    `json:"ktp_document"`
	NpwpDocument       *string    `json:"npwp_document"`
	SelfieDocument     *string    `json:"selfie_document"`
	SelfieKtpDocument  *string    `json:"selfie_ktp_document"`
	PksDocument        *string    `json:"pks_document"`
	CollateralDocument *string    `json:"collateral_document"`
	PlacementBujpName  *string    `json:"placement_bujp_name"`
	PlacementAddress   *string    `json:"placement_address"`
	PlacementDuration  *string    `json:"placement_duration"`
	SubmitImmediately  bool       `json:"submit_immediately"`
}

type UpdateLoanRequest struct {
	Purpose            *string  `json:"purpose"`
	LoanAmount         *float64 `json:"loan_amount" validate:"omitempty,gt=0"`
	TenorMonths        *int     `json:"tenor_months" validate:"omitempty,gte=1,lte=120"`
	DeductFromPayroll  *bool    `json:"deduct_from_payroll"`
	KtpDocument        *string  `json:"ktp_document"`
	NpwpDocument       *string  `json:"npwp_document"`
	SelfieDocument     *string  `json:"selfie_document"`
	SelfieKtpDocument  *string  `json:"selfie_ktp_document"`
	PksDocument        *string  `json:"pks_document"`
	CollateralDocument *string  `json:"collateral_document"`
}

type LoanResponse struct {
	ID                       uuid.UUID                 `json:"id"`
	LoanNumber               string                    `json:"loan_number"`
	PersonnelID              uuid.UUID                 `json:"personnel_id"`
	BujpID                   *uuid.UUID                `json:"bujp_id"`
	LoanProductID            *uuid.UUID                `json:"loan_product_id,omitempty"`
	Purpose                  string                    `json:"purpose"`
	LoanAmount               float64                   `json:"loan_amount"`
	InterestRate             float64                   `json:"interest_rate"`
	TenorMonths              int                       `json:"tenor_months"`
	MonthlyInstallment       float64                   `json:"monthly_installment"`
	TotalRepayment           float64                   `json:"total_repayment"`
	ApprovedAmount           *float64                  `json:"approved_amount,omitempty"`
	ApprovedTenor            *int                      `json:"approved_tenor,omitempty"`
	DisbursementDate         *string                   `json:"disbursement_date,omitempty"`
	FirstInstallmentDate     *string                   `json:"first_installment_date,omitempty"`
	DeductFromPayroll        bool                      `json:"deduct_from_payroll"`
	KtpDocument              *string                   `json:"ktp_document,omitempty"`
	KtpDocumentURL           *string                   `json:"ktp_document_url,omitempty"`
	NpwpDocument             *string                   `json:"npwp_document,omitempty"`
	NpwpDocumentURL          *string                   `json:"npwp_document_url,omitempty"`
	SelfieDocument           *string                   `json:"selfie_document,omitempty"`
	SelfieDocumentURL        *string                   `json:"selfie_document_url,omitempty"`
	SelfieKtpDocument        *string                   `json:"selfie_ktp_document,omitempty"`
	SelfieKtpDocumentURL     *string                   `json:"selfie_ktp_document_url,omitempty"`
	PksDocument              *string                   `json:"pks_document,omitempty"`
	PksDocumentURL           *string                   `json:"pks_document_url,omitempty"`
	CollateralDocument       *string                   `json:"collateral_document,omitempty"`
	CollateralDocumentURL    *string                   `json:"collateral_document_url,omitempty"`
	PlacementCompanyName     *string                   `json:"placement_company_name,omitempty"`
	PlacementBujpName        *string                   `json:"placement_bujp_name,omitempty"`
	PlacementAddress         *string                   `json:"placement_address,omitempty"`
	PlacementDuration        *string                   `json:"placement_duration,omitempty"`
	PlacementLocation        *string                   `json:"placement_location,omitempty"`
	Status                   string                    `json:"status"`
	SubmittedAt              *time.Time                `json:"submitted_at,omitempty"`
	ApprovedAt               *time.Time                `json:"approved_at,omitempty"`
	ApprovedPusatAt          *time.Time                `json:"approved_pusat_at,omitempty"`
	ApprovedBprksAt          *time.Time                `json:"approved_bprks_at,omitempty"`
	RejectedAt               *time.Time                `json:"rejected_at,omitempty"`
	RejectionReason          *string                   `json:"rejection_reason,omitempty"`
	UserConfirmationDeadline *time.Time                `json:"user_confirmation_deadline,omitempty"`
	UserConfirmedAt          *time.Time                `json:"user_confirmed_at,omitempty"`
	CompletedAt              *time.Time                `json:"completed_at,omitempty"`
	CreatedBy                *uuid.UUID                `json:"created_by,omitempty"`
	Personnel                *PersonnelResponse        `json:"personnel,omitempty"`
	Bujp                     *BujpResponse             `json:"bujp,omitempty"`
	LoanProduct              *LoanProductResponse      `json:"loan_product,omitempty"`
	Approvals                []LoanApprovalResponse    `json:"approvals,omitempty"`
	Installments             []LoanInstallmentResponse `json:"installments,omitempty"`
	CreatedAt                time.Time                 `json:"created_at"`
	UpdatedAt                time.Time                 `json:"updated_at"`
}

type LoanUserConfirmationRequest struct {
	Action string  `json:"action" validate:"required,oneof=accept accepted decline declined reject rejected"`
	Reason *string `json:"reason"`
}
