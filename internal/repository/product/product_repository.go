package product

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	FindAll(ctx context.Context, page, limit int) ([]model.Product, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Create(ctx context.Context, product *model.Product) error
	Update(ctx context.Context, product *model.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) withContext(ctx context.Context) *gorm.DB {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return r.db.WithContext(ctxWithTimeout)
}

func (r *repository) FindAll(ctx context.Context, page, limit int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	if err := r.withContext(ctx).Model(&model.Product{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.withContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error
	return products, total, err
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	var product model.Product
	if err := r.withContext(ctx).Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *repository) Create(ctx context.Context, product *model.Product) error {
	return r.withContext(ctx).Create(product).Error
}

func (r *repository) Update(ctx context.Context, product *model.Product) error {
	return r.withContext(ctx).Save(product).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.Product{}).Error
}
