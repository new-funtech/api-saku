package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Location struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID           uuid.UUID `json:"bujp_id" gorm:"type:uuid;not null;index"`
	Code             string    `json:"code" gorm:"type:varchar(20);not null"`
	Name             string    `json:"name" gorm:"type:varchar(200);not null;index"`
	Address          string    `json:"address" gorm:"type:text;not null"`
	Latitude         *float64  `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude        *float64  `json:"longitude" gorm:"type:decimal(11,8)"`
	AttendanceRadius int       `json:"attendance_radius" gorm:"default:100"`
	GuardsNeeded     int       `json:"guards_needed" gorm:"default:1"`
	PicName          *string   `json:"pic_name" gorm:"type:varchar(100)"`
	PicPhone         *string   `json:"pic_phone" gorm:"type:varchar(20)"`
	Status           string    `json:"status" gorm:"type:varchar(20);default:'active';index"`
	CreatedAt        time.Time `json:"created_at" gorm:"index"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	Bujp *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE"`
}

func (l *Location) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type CreateLocationRequest struct {
	BujpID           uuid.UUID `json:"bujp_id" validate:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code             string    `json:"code" validate:"required,max=20" example:"LOC001"`
	Name             string    `json:"name" validate:"required,max=200" example:"Bank BJB Kantor Pusat"`
	Address          string    `json:"address" validate:"required" example:"Jl. Naripan No. 12-14, Bandung"`
	Latitude         *float64  `json:"latitude" example:"-6.917464"`
	Longitude        *float64  `json:"longitude" example:"107.609810"`
	AttendanceRadius int       `json:"attendance_radius" validate:"omitempty,min=10,max=1000" example:"100"`
	GuardsNeeded     int       `json:"guards_needed" validate:"omitempty,min=1" example:"2"`
	PicName          *string   `json:"pic_name" example:"Jane Doe"`
	PicPhone         *string   `json:"pic_phone" example:"081234567890"`
	Status           string    `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type UpdateLocationRequest struct {
	BujpID           *uuid.UUID `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code             *string    `json:"code" validate:"omitempty,max=20" example:"LOC001"`
	Name             *string    `json:"name" validate:"omitempty,max=200" example:"Bank BJB Kantor Pusat"`
	Address          *string    `json:"address" example:"Jl. Naripan No. 12-14, Bandung"`
	Latitude         *float64   `json:"latitude" example:"-6.917464"`
	Longitude        *float64   `json:"longitude" example:"107.609810"`
	AttendanceRadius *int       `json:"attendance_radius" validate:"omitempty,min=10,max=1000" example:"100"`
	GuardsNeeded     *int       `json:"guards_needed" validate:"omitempty,min=1" example:"2"`
	PicName          *string    `json:"pic_name" example:"Jane Doe"`
	PicPhone         *string    `json:"pic_phone" example:"081234567890"`
	Status           *string    `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type LocationResponse struct {
	ID               uuid.UUID     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	BujpID           uuid.UUID     `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code             string        `json:"code" example:"LOC001"`
	Name             string        `json:"name" example:"Bank BJB Kantor Pusat"`
	Address          string        `json:"address" example:"Jl. Naripan No. 12-14, Bandung"`
	Latitude         *float64      `json:"latitude,omitempty" example:"-6.917464"`
	Longitude        *float64      `json:"longitude,omitempty" example:"107.609810"`
	AttendanceRadius int           `json:"attendance_radius" example:"100"`
	GuardsNeeded     int           `json:"guards_needed" example:"2"`
	PicName          *string       `json:"pic_name,omitempty" example:"Jane Doe"`
	PicPhone         *string       `json:"pic_phone,omitempty" example:"081234567890"`
	Status           string        `json:"status" example:"active"`
	Bujp             *BujpResponse `json:"bujp,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}
