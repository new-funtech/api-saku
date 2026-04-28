package repository

import (
	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductRepository interface {
	FindAll(ctx context.Context, page, limit int) ([]model.Product, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Create(ctx context.Context, product *model.Product) error
	Update(ctx context.Context, product *model.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type productRepositoryImpl struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepositoryImpl{db: db}
}

func (r *productRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *productRepositoryImpl) FindAll(ctx context.Context, page, limit int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	if err := r.withContext(ctx).Model(&model.Product{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.withContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error
	return products, total, err
}

func (r *productRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	var product model.Product
	if err := r.withContext(ctx).Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepositoryImpl) Create(ctx context.Context, product *model.Product) error {
	return r.withContext(ctx).Create(product).Error
}

func (r *productRepositoryImpl) Update(ctx context.Context, product *model.Product) error {
	return r.withContext(ctx).Save(product).Error
}

func (r *productRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Product{}).Error
}
