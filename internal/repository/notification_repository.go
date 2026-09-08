package repository

import (
	"time"

	"context"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepository interface {
	WithTx(tx *gorm.DB) NotificationRepository
	Create(ctx context.Context, n *model.Notification) (created bool, err error)
	FindByRecipient(ctx context.Context, recipientUserID uuid.UUID, page, limit int) ([]model.Notification, int64, error)
	CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, recipientUserID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error
	Delete(ctx context.Context, recipientUserID, id uuid.UUID) error
	ClearRead(ctx context.Context, recipientUserID uuid.UUID) error
}

type notificationRepositoryImpl struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepositoryImpl{db: db}
}

func (r *notificationRepositoryImpl) WithTx(tx *gorm.DB) NotificationRepository {
	return &notificationRepositoryImpl{db: tx}
}

func (r *notificationRepositoryImpl) Create(ctx context.Context, n *model.Notification) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}, {Name: "recipient_user_id"}, {Name: "type"}},
			DoNothing: true,
		}).
		Create(n)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *notificationRepositoryImpl) FindByRecipient(ctx context.Context, recipientUserID uuid.UUID, page, limit int) ([]model.Notification, int64, error) {
	var items []model.Notification
	var total int64
	q := r.db.WithContext(ctx).Model(&model.Notification{}).Where("recipient_user_id = ?", recipientUserID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *notificationRepositoryImpl) CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_user_id = ? AND is_read = ?", recipientUserID, false).
		Count(&count).Error
	return count, err
}

func (r *notificationRepositoryImpl) MarkRead(ctx context.Context, recipientUserID, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND recipient_user_id = ?", id, recipientUserID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *notificationRepositoryImpl) MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("recipient_user_id = ? AND is_read = ?", recipientUserID, false).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *notificationRepositoryImpl) Delete(ctx context.Context, recipientUserID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND recipient_user_id = ?", id, recipientUserID).
		Delete(&model.Notification{}).Error
}

func (r *notificationRepositoryImpl) ClearRead(ctx context.Context, recipientUserID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("recipient_user_id = ? AND is_read = ?", recipientUserID, true).
		Delete(&model.Notification{}).Error
}
