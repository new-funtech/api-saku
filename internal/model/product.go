package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Product struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	Name        string         `json:"name" gorm:"type:varchar(255);not null"`
	Description string         `json:"description" gorm:"type:text"`
	Price       float64        `json:"price" gorm:"type:decimal(10,2);not null"`
	Stock       int            `json:"stock" gorm:"type:int;not null;default:0"`
	Image       string         `json:"image" gorm:"type:varchar(500)"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required" example:"Laptop Gaming"`
	Description string  `json:"description" example:"High performance gaming laptop"`
	Price       float64 `json:"price" validate:"required,gt=0" example:"15000000"`
	Stock       int     `json:"stock" validate:"required,gte=0" example:"50"`
	Image       string  `json:"image" example:"products/laptop-gaming.jpg"`
}

type UpdateProductRequest struct {
	Name        string   `json:"name" example:"Laptop Gaming Pro"`
	Description string   `json:"description" example:"Updated high performance gaming laptop"`
	Price       *float64 `json:"price" example:"17000000"`
	Stock       *int     `json:"stock" example:"30"`
	Image       string   `json:"image" example:"products/laptop-gaming-pro.jpg"`
}

type ProductResponse struct {
	ID          uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string    `json:"name" example:"Laptop Gaming"`
	Description string    `json:"description" example:"High performance gaming laptop"`
	Price       float64   `json:"price" example:"15000000"`
	Stock       int       `json:"stock" example:"50"`
	Image       string    `json:"image" example:"products/laptop-gaming.jpg"`
	ImageURL    string    `json:"image_url" example:"https://storage.ganipedia.xyz/ganipedia/products/laptop-gaming.jpg"`
}

type PaginationMeta struct {
	Page        int   `json:"page" example:"1"`
	Limit       int   `json:"limit" example:"10"`
	Total       int64 `json:"total" example:"100"`
	TotalPages  int   `json:"total_pages" example:"10"`
	HasNext     bool  `json:"has_next" example:"true"`
	HasPrevious bool  `json:"has_previous" example:"false"`
}

type APIResponse struct {
	Status  string          `json:"status" example:"success"`
	Code    int             `json:"code" example:"200"`
	Message string          `json:"message" example:"Success"`
	Data    interface{}     `json:"data,omitempty"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}
