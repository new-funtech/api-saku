package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Personnel represents security guards/personnel
type Personnel struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID           uuid.UUID  `json:"bujp_id" gorm:"type:uuid;not null;index"`
	UserID           *uuid.UUID `json:"user_id" gorm:"type:uuid;index"`
	IDNumber         string     `json:"id_number" gorm:"type:varchar(16);not null"`
	NIP              *string    `json:"nip" gorm:"type:varchar(30);index"`
	FullName         string     `json:"full_name" gorm:"type:varchar(100);not null;index"`
	MotherMaidenName *string    `json:"mother_maiden_name" gorm:"type:varchar(100)"`
	Photo            *string    `json:"photo" gorm:"type:varchar(255)"`
	BirthDate        *time.Time `json:"birth_date" gorm:"type:date"`
	Gender           *string    `json:"gender" gorm:"type:varchar(1)"`
	Address          *string    `json:"address" gorm:"type:text"`
	Phone            *string    `json:"phone" gorm:"type:varchar(20);index"`
	EmergencyPhone   *string    `json:"emergency_phone" gorm:"type:varchar(20)"`
	Email            *string    `json:"email" gorm:"type:varchar(100);index"`
	LicenseNumber    *string    `json:"license_number" gorm:"type:varchar(50)"`
	JoinDate         time.Time  `json:"join_date" gorm:"type:date;not null"`
	ContractEndDate  *time.Time `json:"contract_end_date" gorm:"type:date"`
	Status           string     `json:"status" gorm:"type:varchar(20);default:'active';index"`
	BankName         *string    `json:"bank_name" gorm:"type:varchar(50)"`
	AccountNumber    *string    `json:"account_number" gorm:"type:varchar(50)"`
	BaseSalary       float64    `json:"base_salary" gorm:"type:decimal(12,2);default:0"`
	CreatedAt        time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Relations
	Bujp *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE"`
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (p *Personnel) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Request/Response DTOs
type CreatePersonnelRequest struct {
	BujpID           uuid.UUID   `json:"bujp_id" validate:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID           *uuid.UUID  `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	IDNumber         string      `json:"id_number" validate:"required,max=16" example:"3201234567890001"`
	NIP              *string     `json:"nip" validate:"required,max=30" example:"19850101201001001"`
	FullName         string      `json:"full_name" validate:"required,max=100" example:"Ahmad Suryadi"`
	MotherMaidenName *string     `json:"mother_maiden_name" example:"Siti Aminah"`
	Photo            *string     `json:"photo" example:"personnel/photo.jpg"`
	BirthDate        *CustomDate `json:"birth_date" example:"1990-01-15" swaggertype:"string"`
	Gender           *string     `json:"gender" validate:"omitempty,oneof=M F" example:"M"`
	Address          *string     `json:"address" example:"Jl. Merdeka No. 123, Bandung"`
	Phone            *string     `json:"phone" example:"081234567890"`
	EmergencyPhone   *string     `json:"emergency_phone" example:"081234567891"`
	Email            *string     `json:"email" validate:"omitempty,email" example:"ahmad@example.com"`
	LicenseNumber    *string     `json:"license_number" example:"LIC123456"`
	JoinDate         CustomDate  `json:"join_date" validate:"required" example:"2024-01-01" swaggertype:"string"`
	ContractEndDate  *CustomDate `json:"contract_end_date" example:"2025-01-01" swaggertype:"string"`
	Status           string      `json:"status" validate:"omitempty,oneof=active on_leave inactive resigned" example:"active"`
	BankName         *string     `json:"bank_name" example:"Bank BJB"`
	AccountNumber    *string     `json:"account_number" example:"0123456789"`
	BaseSalary       string      `json:"base_salary" example:"5000000"`
	PhotoTempPath    *string     `json:"photo_temp_path,omitempty" example:"PERSONNEL/PHOTOS/temp/abc.jpg"`
	PhotoFinalPath   *string     `json:"photo_final_path,omitempty" example:"PERSONNEL/PHOTOS/{personnel_id}/abc.jpg"`
}

type UpdatePersonnelRequest struct {
	BujpID           *uuid.UUID  `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID           *uuid.UUID  `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	IDNumber         *string     `json:"id_number" validate:"omitempty,max=16" example:"3201234567890001"`
	NIP              *string     `json:"nip" validate:"omitempty,max=30" example:"19850101201001001"`
	FullName         *string     `json:"full_name" validate:"omitempty,max=100" example:"Ahmad Suryadi"`
	MotherMaidenName *string     `json:"mother_maiden_name" example:"Siti Aminah"`
	Photo            *string     `json:"photo" example:"personnel/photo.jpg"`
	BirthDate        *CustomDate `json:"birth_date" example:"1990-01-15" swaggertype:"string"`
	Gender           *string     `json:"gender" validate:"omitempty,oneof=M F" example:"M"`
	Address          *string     `json:"address" example:"Jl. Merdeka No. 123, Bandung"`
	Phone            *string     `json:"phone" example:"081234567890"`
	EmergencyPhone   *string     `json:"emergency_phone" example:"081234567891"`
	Email            *string     `json:"email" validate:"omitempty,email" example:"ahmad@example.com"`
	LicenseNumber    *string     `json:"license_number" example:"LIC123456"`
	JoinDate         *CustomDate `json:"join_date" example:"2024-01-01" swaggertype:"string"`
	ContractEndDate  *CustomDate `json:"contract_end_date" example:"2025-01-01" swaggertype:"string"`
	Status           *string     `json:"status" validate:"omitempty,oneof=active on_leave inactive resigned" example:"active"`
	BankName         *string     `json:"bank_name" example:"Bank BJB"`
	AccountNumber    *string     `json:"account_number" example:"0123456789"`
	BaseSalary       *string     `json:"base_salary" example:"5000000"`
	PhotoTempPath    *string     `json:"photo_temp_path,omitempty" example:"PERSONNEL/PHOTOS/temp/abc.jpg"`
	PhotoFinalPath   *string     `json:"photo_final_path,omitempty" example:"PERSONNEL/PHOTOS/{personnel_id}/abc.jpg"`
}

type PersonnelResponse struct {
	ID               uuid.UUID     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	BujpID           uuid.UUID     `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID           *uuid.UUID    `json:"user_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	IDNumber         string        `json:"id_number" example:"3201234567890001"`
	NIP              *string       `json:"nip,omitempty" example:"19850101201001001"`
	FullName         string        `json:"full_name" example:"Ahmad Suryadi"`
	MotherMaidenName *string       `json:"mother_maiden_name,omitempty" example:"Siti Aminah"`
	Photo            *string       `json:"photo,omitempty" example:"personnel/photo.jpg"`
	BirthDate        *string       `json:"birth_date,omitempty" example:"1990-01-15"`
	Gender           *string       `json:"gender,omitempty" example:"M"`
	Address          *string       `json:"address,omitempty" example:"Jl. Merdeka No. 123, Bandung"`
	Phone            *string       `json:"phone,omitempty" example:"081234567890"`
	EmergencyPhone   *string       `json:"emergency_phone,omitempty" example:"081234567891"`
	Email            *string       `json:"email,omitempty" example:"ahmad@example.com"`
	LicenseNumber    *string       `json:"license_number,omitempty" example:"LIC123456"`
	JoinDate         string        `json:"join_date" example:"2024-01-01"`
	ContractEndDate  *string       `json:"contract_end_date,omitempty" example:"2025-01-01"`
	Status           string        `json:"status" example:"active"`
	BankName         *string       `json:"bank_name,omitempty" example:"Bank BJB"`
	AccountNumber    *string       `json:"account_number,omitempty" example:"0123456789"`
	BaseSalary       float64       `json:"base_salary" example:"5000000"`
	Bujp             *BujpResponse `json:"bujp,omitempty"`
	User             *UserResponse `json:"user,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// Personnel Import DTOs
type PersonnelImportRow struct {
	Row              int
	Email            string
	FullName         string
	MotherMaidenName string
	IDNumber         string
	Phone            string
	EmergencyPhone   string
	Address          string
	Gender           string
	Status           string
	BirthDate        string
	LicenseNumber    string
	BankName         string
	AccountNumber    string
	JoinDate         string
	ContractEndDate  string
	BaseSalary       string
}

type PersonnelImportResult struct {
	Row      int                    `json:"row"`
	Status   string                 `json:"status"`
	ID       *uuid.UUID             `json:"id,omitempty"`
	FullName string                 `json:"full_name"`
	Message  string                 `json:"message,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type PersonnelImportError struct {
	Row     int                    `json:"row"`
	Field   string                 `json:"field"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type PersonnelImportResponse struct {
	SuccessCount int                     `json:"success_count"`
	ErrorCount   int                     `json:"error_count"`
	Results      []PersonnelImportResult `json:"results"`
	Errors       []PersonnelImportError  `json:"errors"`
}
