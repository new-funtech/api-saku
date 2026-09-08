package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanProduct struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID            *uuid.UUID `json:"bujp_id" gorm:"type:uuid;index"`
	Code              string     `json:"code" gorm:"type:varchar(20);not null"`
	Name              string     `json:"name" gorm:"type:varchar(100);not null"`
	Description       *string    `json:"description" gorm:"type:text"`
	MinAmount         float64    `json:"min_amount" gorm:"type:decimal(12,2);default:0"`
	MaxAmount         float64    `json:"max_amount" gorm:"type:decimal(12,2);default:0"`
	InterestRate      float64    `json:"interest_rate" gorm:"type:decimal(5,2);default:0"`
	MaxTenor          int        `json:"max_tenor" gorm:"default:12"`
	RequirePks        bool       `json:"require_pks" gorm:"default:true"`
	RequireCollateral bool       `json:"require_collateral" gorm:"default:false"`
	IsActive          bool       `json:"is_active" gorm:"default:true;index"`
	CreatedBy         *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	Bujp    *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
	Creator *User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (lp *LoanProduct) BeforeCreate(tx *gorm.DB) error {
	if lp.ID == uuid.Nil {
		lp.ID = uuid.New()
	}
	return nil
}

type CreateLoanProductRequest struct {
	BujpID            *uuid.UUID `json:"bujp_id"`
	Name              string     `json:"name" validate:"required,max=100"`
	Description       *string    `json:"description"`
	MinAmount         float64    `json:"min_amount" validate:"gte=0"`
	MaxAmount         float64    `json:"max_amount" validate:"gtfield=MinAmount"`
	InterestRate      float64    `json:"interest_rate" validate:"gte=0,lte=100"`
	MaxTenor          int        `json:"max_tenor" validate:"required,gte=1,lte=120"`
	RequirePks        *bool      `json:"require_pks"`
	RequireCollateral *bool      `json:"require_collateral"`
	IsActive          *bool      `json:"is_active"`
}

type UpdateLoanProductRequest struct {
	BujpID            *uuid.UUID `json:"bujp_id"`
	Name              *string    `json:"name" validate:"omitempty,max=100"`
	Description       *string    `json:"description"`
	MinAmount         *float64   `json:"min_amount" validate:"omitempty,gte=0"`
	MaxAmount         *float64   `json:"max_amount" validate:"omitempty,gte=0"`
	InterestRate      *float64   `json:"interest_rate" validate:"omitempty,gte=0,lte=100"`
	MaxTenor          *int       `json:"max_tenor" validate:"omitempty,gte=1,lte=120"`
	RequirePks        *bool      `json:"require_pks"`
	RequireCollateral *bool      `json:"require_collateral"`
	IsActive          *bool      `json:"is_active"`
}

type LoanProductResponse struct {
	ID                uuid.UUID     `json:"id"`
	BujpID            *uuid.UUID    `json:"bujp_id,omitempty"`
	Code              string        `json:"code"`
	Name              string        `json:"name"`
	Description       *string       `json:"description,omitempty"`
	MinAmount         float64       `json:"min_amount"`
	MaxAmount         float64       `json:"max_amount"`
	InterestRate      float64       `json:"interest_rate"`
	MaxTenor          int           `json:"max_tenor"`
	RequirePks        bool          `json:"require_pks"`
	RequireCollateral bool          `json:"require_collateral"`
	IsActive          bool          `json:"is_active"`
	CreatedBy         *uuid.UUID    `json:"created_by,omitempty"`
	Bujp              *BujpResponse `json:"bujp,omitempty"`
	Creator           *UserResponse `json:"creator,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}
