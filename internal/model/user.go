package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BujpID    *uuid.UUID `json:"bujp_id" gorm:"type:uuid;index"`
	Email     string     `json:"email" gorm:"type:varchar(100);not null"`
	Password  string     `json:"-" gorm:"type:varchar(255);not null"`
	Role      string     `json:"role" gorm:"type:varchar(50);not null;index"`
	FullName  string     `json:"full_name" gorm:"type:varchar(100);not null;index"`
	Phone     *string    `json:"phone" gorm:"type:varchar(20);index"`
	Photo     *string    `json:"photo" gorm:"type:varchar(255)"`
	Status    string     `json:"status" gorm:"type:varchar(20);default:'active';index"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt time.Time  `json:"updated_at"`

	Bujp *Bujp `json:"bujp,omitempty" gorm:"foreignKey:BujpID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type CreateUserRequest struct {
	BujpID   *uuid.UUID `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email    string     `json:"email" validate:"required,email" example:"john@example.com"`
	Password string     `json:"password" validate:"required,min=8,max=72" example:"password123"`
	Role     string     `json:"role" validate:"required,oneof=super_admin company_admin supervisor guard" example:"guard"`
	FullName string     `json:"full_name" validate:"required,max=100" example:"John Doe"`
	Phone    *string    `json:"phone" example:"081234567890"`
	Photo    *string    `json:"photo" example:"users/photo.jpg"`
	Status   string     `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type UpdateUserRequest struct {
	BujpID   *uuid.UUID `json:"bujp_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email    *string    `json:"email" validate:"omitempty,email" example:"john@example.com"`
	Password *string    `json:"password,omitempty" validate:"omitempty,min=8,max=72" example:"newpassword123"`
	Role     *string    `json:"role" validate:"omitempty,oneof=super_admin company_admin supervisor guard" example:"guard"`
	FullName *string    `json:"full_name" validate:"omitempty,max=100" example:"John Doe"`
	Phone    *string    `json:"phone" example:"081234567890"`
	Photo    *string    `json:"photo" example:"users/photo.jpg"`
	Status   *string    `json:"status" validate:"omitempty,oneof=active inactive" example:"active"`
}

type UpdateProfileRequest struct {
	FullName *string `json:"full_name" validate:"omitempty,max=100" example:"John Doe"`
	Phone    *string `json:"phone" example:"081234567890"`
	Photo    *string `json:"photo" example:"users/photo.jpg"`
	Email    *string `json:"email" validate:"omitempty,email" example:"john@example.com"`
	Password *string `json:"password,omitempty" validate:"omitempty,min=8,max=72" example:"newpassword123"`
}

type ProfileResponse struct {
	ID                uuid.UUID                `json:"id"`
	Email             string                   `json:"email"`
	Role              string                   `json:"role"`
	FullName          string                   `json:"full_name"`
	Phone             *string                  `json:"phone,omitempty"`
	Photo             *string                  `json:"photo,omitempty"`
	Status            string                   `json:"status"`
	BujpID            *uuid.UUID               `json:"bujp_id,omitempty"`
	BujpName          *string                  `json:"bujp_name,omitempty"`
	Personnel         *PersonnelResponse       `json:"personnel,omitempty"`
	CurrentAssignment *CurrentAssignmentDetail `json:"current_assignment,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type CurrentAssignmentDetail struct {
	ID        uuid.UUID       `json:"id"`
	StartDate *time.Time      `json:"start_date"`
	EndDate   *time.Time      `json:"end_date"`
	Status    string          `json:"status"`
	Notes     *string         `json:"notes,omitempty"`
	Shift     *ShiftDetail    `json:"shift,omitempty"`
	Location  *LocationDetail `json:"location,omitempty"`
}

type ShiftDetail struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	StartTime TimeOnly  `json:"start_time" swaggertype:"string"`
	EndTime   TimeOnly  `json:"end_time" swaggertype:"string"`
}

type LocationDetail struct {
	ID               uuid.UUID `json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Address          *string   `json:"address,omitempty"`
	Latitude         *float64  `json:"latitude,omitempty"`
	Longitude        *float64  `json:"longitude,omitempty"`
	AttendanceRadius *int      `json:"attendance_radius,omitempty"`
	BujpName         *string   `json:"bujp_name,omitempty"`
}

type UserResponse struct {
	ID        uuid.UUID     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	BujpID    *uuid.UUID    `json:"bujp_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string        `json:"email" example:"john@example.com"`
	Role      string        `json:"role" example:"guard"`
	FullName  string        `json:"full_name" example:"John Doe"`
	Phone     *string       `json:"phone,omitempty" example:"081234567890"`
	Photo     *string       `json:"photo,omitempty" example:"users/photo.jpg"`
	Status    string        `json:"status" example:"active"`
	LastLogin *time.Time    `json:"last_login,omitempty"`
	Bujp      *BujpResponse `json:"bujp,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required" example:"oldpassword123"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72" example:"newpassword123"`
	ConfirmPassword string `json:"new_password_confirmation" validate:"required,eqfield=NewPassword" example:"newpassword123"`
}
