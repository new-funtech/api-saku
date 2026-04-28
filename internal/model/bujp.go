package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Bujp represents the BUJP (security provider company) entity
type Bujp struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	Code             string     `json:"code" gorm:"type:varchar(20);not null"`
	Name             string     `json:"name" gorm:"type:varchar(200);not null"`
	Address          *string    `json:"address" gorm:"type:text"`
	Phone            *string    `json:"phone" gorm:"type:varchar(20)"`
	Email            *string    `json:"email" gorm:"type:varchar(100)"`
	PicName          *string    `json:"pic_name" gorm:"type:varchar(100)"`
	PicPhone         *string    `json:"pic_phone" gorm:"type:varchar(20)"`
	PksNumber        *string    `json:"pks_number" gorm:"type:varchar(100)"`
	PksValidityStart *time.Time `json:"pks_validity_start" gorm:"type:date"`
	PksValidityEnd   *time.Time `json:"pks_validity_end" gorm:"type:date"`
	PksDocument      *string    `json:"pks_document" gorm:"type:varchar(255)"`
	BjbAccountNumber *string    `json:"bjb_account_number" gorm:"type:varchar(50)"`
	Status           string     `json:"status" gorm:"type:varchar(20);default:'active'"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Relations
	Users      []User      `json:"users,omitempty" gorm:"foreignKey:BujpID"`
	Locations  []Location  `json:"locations,omitempty" gorm:"foreignKey:BujpID"`
	Shifts     []Shift     `json:"shifts,omitempty" gorm:"foreignKey:BujpID"`
	Personnels []Personnel `json:"personnels,omitempty" gorm:"foreignKey:BujpID"`
}

func (b *Bujp) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreateBujpRequest struct {
	Code             string  `json:"code" validate:"required,max=20" example:"BUJP001"`
	Name             string  `json:"name" validate:"required,max=200" example:"GARDA MUTIARA TIMUR"`
	Address          *string `json:"address" example:"Jl. Kebun Mas Blok Q147"`
	Phone            *string `json:"phone" example:"081234567890"`
	Email            *string `json:"email" validate:"omitempty,email" example:"info@bujp.com"`
	PicName          *string `json:"pic_name" example:"John Doe"`
	PicPhone         *string `json:"pic_phone" example:"081234567890"`
	PksNumber        *string `json:"pks_number" example:"PKS/BJB/2026/001"`
	PksValidityStart *string `json:"pks_validity_start" example:"2026-01-01"` // format: 2006-01-02
	PksValidityEnd   *string `json:"pks_validity_end" example:"2027-01-01"`   // format: 2006-01-02
	PksDocument      *string `json:"pks_document" example:"bujps/pks/doc.pdf"`
	BjbAccountNumber *string `json:"bjb_account_number" example:"0123456789"`
	Status           string  `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type UpdateBujpRequest struct {
	Code             *string `json:"code" validate:"omitempty,max=20" example:"BUJP001"`
	Name             *string `json:"name" validate:"omitempty,max=200" example:"GARDA MUTIARA TIMUR"`
	Address          *string `json:"address" example:"Jl. Kebun Mas Blok Q147"`
	Phone            *string `json:"phone" example:"081234567890"`
	Email            *string `json:"email" validate:"omitempty,email" example:"info@bujp.com"`
	PicName          *string `json:"pic_name" example:"John Doe"`
	PicPhone         *string `json:"pic_phone" example:"081234567890"`
	PksNumber        *string `json:"pks_number" example:"PKS/BJB/2026/001"`
	PksValidityStart *string `json:"pks_validity_start" example:"2026-01-01"` // format: 2006-01-02
	PksValidityEnd   *string `json:"pks_validity_end" example:"2027-01-01"`   // format: 2006-01-02
	PksDocument      *string `json:"pks_document" example:"bujps/pks/doc.pdf"`
	BjbAccountNumber *string `json:"bjb_account_number" example:"0123456789"`
	Status           *string `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type BujpResponse struct {
	ID               uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code             string     `json:"code" example:"BUJP001"`
	Name             string     `json:"name" example:"GARDA MUTIARA TIMUR"`
	Address          *string    `json:"address,omitempty" example:"Jl. Kebun Mas Blok Q147"`
	Phone            *string    `json:"phone,omitempty" example:"081234567890"`
	Email            *string    `json:"email,omitempty" example:"info@bujp.com"`
	PicName          *string    `json:"pic_name,omitempty" example:"John Doe"`
	PicPhone         *string    `json:"pic_phone,omitempty" example:"081234567890"`
	PksNumber        *string    `json:"pks_number,omitempty" example:"PKS/BJB/2026/001"`
	PksValidityStart *time.Time `json:"pks_validity_start,omitempty" example:"2026-01-01"`
	PksValidityEnd   *time.Time `json:"pks_validity_end,omitempty" example:"2027-01-01"`
	PksDocument      *string    `json:"pks_document,omitempty" example:"bujps/pks/doc.pdf"`
	PksDocumentUrl   *string    `json:"pks_document_url,omitempty" example:"https://s3.example.com/presigned-url"`
	BjbAccountNumber *string    `json:"bjb_account_number,omitempty" example:"0123456789"`
	Status           string     `json:"status" example:"active"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
