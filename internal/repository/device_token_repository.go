package repository

import (
	"context"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeviceTokenRepository interface {
	// Upsert stores the token for the user. A token already held by another user moves to this one
	// (the same phone signed in with a different account), so it never notifies the previous owner.
	Upsert(ctx context.Context, userID uuid.UUID, fcmToken, platform string, deviceID *string) error
	// DeleteForUser removes a token, but only when it belongs to the user.
	DeleteForUser(ctx context.Context, userID uuid.UUID, fcmToken string) (bool, error)
	// TokensByUser lists every token of the user.
	TokensByUser(ctx context.Context, userID uuid.UUID) ([]string, error)
	// DeleteTokens removes tokens FCM reported as dead.
	DeleteTokens(ctx context.Context, tokens []string) error
	// PushEnabled is the user's push switch; on when they never changed it. A lookup error also counts
	// as on: a missed setting must not silence notifications.
	PushEnabled(ctx context.Context, userID uuid.UUID) bool
}

type deviceTokenRepositoryImpl struct {
	db *gorm.DB
}

func NewDeviceTokenRepository(db *gorm.DB) DeviceTokenRepository {
	return &deviceTokenRepositoryImpl{db: db}
}

func (r *deviceTokenRepositoryImpl) Upsert(ctx context.Context, userID uuid.UUID, fcmToken, platform string, deviceID *string) error {
	row := &model.DeviceToken{UserID: userID, FCMToken: fcmToken, Platform: platform, DeviceID: deviceID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fcm_token"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"user_id": userID, "platform": platform, "device_id": deviceID, "updated_at": time.Now()}),
	}).Create(row).Error
}

func (r *deviceTokenRepositoryImpl) DeleteForUser(ctx context.Context, userID uuid.UUID, fcmToken string) (bool, error) {
	res := r.db.WithContext(ctx).Where("user_id = ? AND fcm_token = ?", userID, fcmToken).Delete(&model.DeviceToken{})
	return res.RowsAffected > 0, res.Error
}

func (r *deviceTokenRepositoryImpl) TokensByUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var tokens []string
	err := r.db.WithContext(ctx).Model(&model.DeviceToken{}).Where("user_id = ?", userID).Pluck("fcm_token", &tokens).Error
	return tokens, err
}

func (r *deviceTokenRepositoryImpl) DeleteTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("fcm_token IN ?", tokens).Delete(&model.DeviceToken{}).Error
}

func (r *deviceTokenRepositoryImpl) PushEnabled(ctx context.Context, userID uuid.UUID) bool {
	var p model.NotificationPreference
	err := r.db.WithContext(ctx).Select("push_enabled").Where("user_id = ?", userID).Take(&p).Error
	if err != nil {
		return true
	}
	return p.PushEnabled
}
