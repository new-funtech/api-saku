package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.User, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByIDWithRelations(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *userRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.withContext(ctx).Model(&model.User{})

	// Apply filters
	if excludeUserID, ok := filters["exclude_user_id"].(uuid.UUID); ok && excludeUserID != uuid.Nil {
		query = query.Where("id != ?", excludeUserID)
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Where("bujp_id = ?", bujpID)
	}
	if role, ok := filters["role"].(string); ok && role != "" {
		query = query.Where("role = ?", role)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("full_name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query = query.Preload("Bujp", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "code", "name")
	})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Select("id", "bujp_id", "email", "role", "full_name", "phone", "photo", "status", "last_login", "created_at", "updated_at").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

func (r *userRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := r.withContext(ctx).Preload("Bujp").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByIDWithRelations(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := r.withContext(ctx).Preload("Bujp").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.withContext(ctx).Preload("Bujp").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.User, error) {
	var users []model.User
	err := r.withContext(ctx).Where("bujp_id = ?", bujpID).Find(&users).Error
	return users, err
}

func (r *userRepositoryImpl) Create(ctx context.Context, user *model.User) error {
	return r.withContext(ctx).Create(user).Error
}

func (r *userRepositoryImpl) Update(ctx context.Context, user *model.User) error {
	// Use Save() so pointer fields cleared to nil (e.g. bujp_id) and other
	// changes are persisted reliably. Updates() with a struct skips zero values
	// which silently dropped legitimate updates such as switching BUJP.
	return r.withContext(ctx).Save(user).Error
}

func (r *userRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// Hard delete: models no longer use gorm.DeletedAt so this issues a real
	// DELETE statement.
	return r.withContext(ctx).Where("id = ?", id).Delete(&model.User{}).Error
}

func (r *userRepositoryImpl) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.withContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("last_login", now).Error
}
