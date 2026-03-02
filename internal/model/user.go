package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string         `json:"name" gorm:"type:varchar(255);not null"`
	Email     string         `json:"email" gorm:"type:varchar(255);not null"`
	Password  string         `json:"-" gorm:"type:varchar(255);not null"`
	Role      string         `json:"role" gorm:"type:varchar(50);not null;default:'user'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required,min=6" example:"password123"`
	Role     string `json:"role" example:"user"`
}

type UpdateUserRequest struct {
	Name     string `json:"name" example:"John Smith"`
	Email    string `json:"email" example:"johnsmith@example.com"`
	Password string `json:"password,omitempty" example:"newpassword123"`
	Role     string `json:"role" example:"admin"`
}

type UserResponse struct {
	ID    uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name  string    `json:"name" example:"John Doe"`
	Email string    `json:"email" example:"john@example.com"`
	Role  string    `json:"role" example:"user"`
}
