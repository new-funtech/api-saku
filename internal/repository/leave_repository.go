package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LeaveRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Leave, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Leave, error)
	FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Leave, int64, error)
	FindPending(ctx context.Context, page, limit int) ([]model.Leave, int64, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time, personnelID uuid.UUID) ([]model.Leave, error)
	Create(ctx context.Context, leave *model.Leave) error
	Update(ctx context.Context, leave *model.Leave) error
	Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type leaveRepositoryImpl struct {
	db *gorm.DB
}

func NewLeaveRepository(db *gorm.DB) LeaveRepository {
	return &leaveRepositoryImpl{db: db}
}

func (r *leaveRepositoryImpl) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

func (r *leaveRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Leave, int64, error) {
	db := r.withContext(ctx)

	var leaves []model.Leave
	var total int64

	query := db.Model(&model.Leave{})

	// Tenant scoping.
	if v, ok := filters["force_empty"].(bool); ok && v {
		return []model.Leave{}, 0, nil
	}
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		query = query.Joins("JOIN personnels ON personnels.id = leaves.personnel_id").
			Where("personnels.bujp_id = ?", bujpID)
	}

	// Apply filters
	if personnelID, ok := filters["personnel_id"].(uuid.UUID); ok && personnelID != uuid.Nil {
		query = query.Where("personnel_id = ?", personnelID)
	}
	if leaveType, ok := filters["type"].(string); ok && leaveType != "" {
		query = query.Where("type = ?", leaveType)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Approver").
		Order("start_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&leaves).Error

	return leaves, total, err
}

func (r *leaveRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Leave, error) {
	db := r.withContext(ctx)

	var leave model.Leave
	err := db.
		Preload("Personnel").
		Preload("Approver").
		First(&leave, "id = ?", id).Error

	if err != nil {
		return nil, err
	}
	return &leave, nil
}

func (r *leaveRepositoryImpl) FindByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.Leave, int64, error) {
	db := r.withContext(ctx)

	var leaves []model.Leave
	var total int64

	query := db.Model(&model.Leave{}).Where("personnel_id = ?", personnelID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Preload("Approver").
		Order("start_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&leaves).Error

	return leaves, total, err
}

func (r *leaveRepositoryImpl) FindPending(ctx context.Context, page, limit int) ([]model.Leave, int64, error) {
	db := r.withContext(ctx)

	var leaves []model.Leave
	var total int64

	query := db.Model(&model.Leave{}).Where("status = ?", "pending")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Personnel").
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&leaves).Error

	return leaves, total, err
}

func (r *leaveRepositoryImpl) FindByDateRange(ctx context.Context, startDate, endDate time.Time, personnelID uuid.UUID) ([]model.Leave, error) {
	db := r.withContext(ctx)

	var leaves []model.Leave
	err := db.
		Where("personnel_id = ? AND status = ?", personnelID, "approved").
		Where("((start_date BETWEEN ? AND ?) OR (end_date BETWEEN ? AND ?) OR (start_date <= ? AND end_date >= ?))",
			startDate, endDate,
			startDate, endDate,
			startDate, endDate).
		Order("start_date ASC").
		Find(&leaves).Error

	return leaves, err
}

func (r *leaveRepositoryImpl) Create(ctx context.Context, leave *model.Leave) error {
	db := r.withContext(ctx)

	return db.Create(leave).Error
}

func (r *leaveRepositoryImpl) Update(ctx context.Context, leave *model.Leave) error {
	db := r.withContext(ctx)

	result := db.Model(leave).Where("id = ?", leave.ID).Updates(leave)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("leave not found")
	}
	return nil
}

func (r *leaveRepositoryImpl) Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) error {
	db := r.withContext(ctx)

	now := time.Now()
	updates := map[string]interface{}{
		"status":         status,
		"approver_id":    approverID,
		"approved_at":    now,
		"approver_notes": notes,
	}

	result := db.Model(&model.Leave{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("leave not found")
	}
	return nil
}

func (r *leaveRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	db := r.withContext(ctx)

	result := db.Delete(&model.Leave{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("leave not found")
	}
	return nil
}
