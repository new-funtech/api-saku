package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OvertimeRepository interface {
	FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Overtime, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Overtime, error)
	// CreateIfFree inserts unless the personnel already has an overlapping pending/approved
	// overtime that day, in which case that one is returned instead.
	CreateIfFree(ctx context.Context, o *model.Overtime) (*model.Overtime, error)
	// UpdatePending writes the changes only while the request is still pending.
	UpdatePending(ctx context.Context, o *model.Overtime) error
	// Decide records the decision only while the request is still pending.
	Decide(ctx context.Context, id, approverID uuid.UUID, status, notes string) error
	// FindApproved returns the approved overtime of a BUJP's personnel between two dates (inclusive).
	FindApproved(ctx context.Context, bujpID uuid.UUID, from, to time.Time) ([]model.Overtime, error)
	// DeleteUnapproved removes a request that has not been approved.
	DeleteUnapproved(ctx context.Context, id uuid.UUID) error
}

type overtimeRepositoryImpl struct{ db *gorm.DB }

func NewOvertimeRepository(db *gorm.DB) OvertimeRepository { return &overtimeRepositoryImpl{db: db} }

func (r *overtimeRepositoryImpl) FindAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.Overtime, int64, error) {
	var items []model.Overtime
	var total int64
	if v, ok := filters["force_empty"].(bool); ok && v {
		return items, 0, nil
	}
	q := r.db.WithContext(ctx).Model(&model.Overtime{})
	if bujpID, ok := filters["bujp_id"].(uuid.UUID); ok && bujpID != uuid.Nil {
		q = q.Joins("JOIN personnels ON personnels.id = overtimes.personnel_id").Where("personnels.bujp_id = ?", bujpID)
	}
	if pid, ok := filters["personnel_id"].(uuid.UUID); ok && pid != uuid.Nil {
		q = q.Where("overtimes.personnel_id = ?", pid)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		q = q.Where("overtimes.status = ?", status)
	}
	if from, ok := filters["date_from"].(time.Time); ok {
		q = q.Where("overtimes.date >= ?", from)
	}
	if to, ok := filters["date_to"].(time.Time); ok {
		q = q.Where("overtimes.date <= ?", to)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "overtimes.date DESC, overtimes.created_at DESC"
	if pending, _ := filters["status"].(string); pending == "pending" {
		order = "overtimes.created_at ASC" // oldest request first for reviewers
	}
	err := q.Preload("Personnel").Preload("Approver").Order(order).
		Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *overtimeRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.Overtime, error) {
	var o model.Overtime
	if err := r.db.WithContext(ctx).Preload("Personnel").Preload("Approver").First(&o, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *overtimeRepositoryImpl) CreateIfFree(ctx context.Context, o *model.Overtime) (*model.Overtime, error) {
	var conflict *model.Overtime
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "overtime:"+o.PersonnelID.String()).Error; err != nil {
			return err
		}
		var existing model.Overtime
		// HH:MM strings compare correctly as text; ranges overlap when each starts before the other ends.
		err := tx.Where("personnel_id = ? AND date = ? AND status IN ? AND start_time < ? AND end_time > ?",
			o.PersonnelID, o.Date, []string{"pending", "approved"}, o.EndTime, o.StartTime).
			Limit(1).Take(&existing).Error
		if err == nil {
			conflict = &existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(o).Error
	})
	return conflict, err
}

func (r *overtimeRepositoryImpl) UpdatePending(ctx context.Context, o *model.Overtime) error {
	res := r.db.WithContext(ctx).Model(&model.Overtime{}).
		Where("id = ? AND status = ?", o.ID, "pending").
		Updates(map[string]interface{}{
			"date": o.Date, "start_time": o.StartTime, "end_time": o.EndTime,
			"total_hours": o.TotalHours, "reason": o.Reason,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotPending
	}
	return nil
}

func (r *overtimeRepositoryImpl) Decide(ctx context.Context, id, approverID uuid.UUID, status, notes string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.Overtime{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status": status, "approver_id": approverID, "approved_at": now, "approver_notes": notes,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotPending
	}
	return nil
}

func (r *overtimeRepositoryImpl) DeleteUnapproved(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND status <> ?", id, "approved").Delete(&model.Overtime{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotPending
	}
	return nil
}

func (r *overtimeRepositoryImpl) FindApproved(ctx context.Context, bujpID uuid.UUID, from, to time.Time) ([]model.Overtime, error) {
	var items []model.Overtime
	err := r.db.WithContext(ctx).Model(&model.Overtime{}).
		Joins("JOIN personnels ON personnels.id = overtimes.personnel_id").
		Where("personnels.bujp_id = ? AND overtimes.status = ? AND overtimes.date >= ? AND overtimes.date <= ?",
			bujpID, "approved", from, to).
		Find(&items).Error
	return items, err
}
