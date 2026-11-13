package repository

import (
	"context"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationPreferenceRepository interface {
	// Get returns the user's switches; a user who never changed them has both on.
	Get(ctx context.Context, userID uuid.UUID) (push, email bool, err error)
	Upsert(ctx context.Context, userID uuid.UUID, push, email bool) error
	// EmailOptedOut returns, from the given addresses, those whose user turned email notifications off
	// (lower-cased). Addresses with no matching user, or no preference, are not in the result.
	EmailOptedOut(ctx context.Context, emails []string) (map[string]bool, error)
}

type notificationPreferenceRepositoryImpl struct{ db *gorm.DB }

func NewNotificationPreferenceRepository(db *gorm.DB) NotificationPreferenceRepository {
	return &notificationPreferenceRepositoryImpl{db: db}
}

func (r *notificationPreferenceRepositoryImpl) Get(ctx context.Context, userID uuid.UUID) (bool, bool, error) {
	var p model.NotificationPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&p).Error
	if err == gorm.ErrRecordNotFound {
		return true, true, nil
	}
	if err != nil {
		return true, true, err
	}
	return p.PushEnabled, p.EmailEnabled, nil
}

func (r *notificationPreferenceRepositoryImpl) Upsert(ctx context.Context, userID uuid.UUID, push, email bool) error {
	row := &model.NotificationPreference{UserID: userID, PushEnabled: push, EmailEnabled: email}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"push_enabled": push, "email_enabled": email, "updated_at": time.Now(),
		}),
	}).Create(row).Error
}

func (r *notificationPreferenceRepositoryImpl) EmailOptedOut(ctx context.Context, emails []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(emails) == 0 {
		return out, nil
	}
	lowered := make([]string, len(emails))
	for i, e := range emails {
		lowered[i] = strings.ToLower(strings.TrimSpace(e))
	}
	var found []string
	err := r.db.WithContext(ctx).Table("users").
		Joins("JOIN notification_preferences p ON p.user_id = users.id").
		Where("p.email_enabled = ? AND lower(users.email) IN ?", false, lowered).
		Pluck("lower(users.email)", &found).Error
	for _, e := range found {
		out[e] = true
	}
	return out, err
}
