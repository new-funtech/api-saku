package product

import (
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	FindAll(page, limit int) ([]model.Product, int64, error)
	FindByID(id uuid.UUID) (*model.Product, error)
	Create(product *model.Product) error
	Update(product *model.Product) error
	Delete(id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindAll(page, limit int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	if err := r.db.Model(&model.Product{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error
	return products, total, err
}

func (r *repository) FindByID(id uuid.UUID) (*model.Product, error) {
	var product model.Product
	if err := r.db.Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *repository) Create(product *model.Product) error {
	return r.db.Create(product).Error
}

func (r *repository) Update(product *model.Product) error {
	return r.db.Save(product).Error
}

func (r *repository) Delete(id uuid.UUID) error {
	return r.db.Where("id = ?", id).Delete(&model.Product{}).Error
}
