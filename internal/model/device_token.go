package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceToken is an FCM registration token of one app install, tied to the user signed in on it.
type DeviceToken struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	FCMToken  string    `json:"fcm_token" gorm:"column:fcm_token;type:text;not null;uniqueIndex"`
	Platform  string    `json:"platform" gorm:"type:varchar(20);not null;default:'android'"`
	DeviceID  *string   `json:"device_id" gorm:"type:varchar(100);index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User *User `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
}

func (d *DeviceToken) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// RegisterDeviceTokenRequest is what the app sends after it obtains (or refreshes) its FCM token.
type RegisterDeviceTokenRequest struct {
	FCMToken string  `json:"fcm_token" validate:"required,min=20,max=4096"`
	Platform string  `json:"platform" validate:"omitempty,oneof=android ios web"`
	DeviceID *string `json:"device_id" validate:"omitempty,max=100"`
}

// UnregisterDeviceTokenRequest removes the token on sign-out.
type UnregisterDeviceTokenRequest struct {
	FCMToken string `json:"fcm_token" validate:"required,min=20,max=4096"`
}
