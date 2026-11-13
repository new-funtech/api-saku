package model

import (
	"time"

	"github.com/google/uuid"
)

// NotificationPreference lets a user switch push and email notifications on or off. A user with no
// row has both on. In-app notifications (the list inside the app) are not affected.
type NotificationPreference struct {
	UserID       uuid.UUID `json:"user_id" gorm:"type:uuid;primaryKey"`
	PushEnabled  bool      `json:"push_enabled" gorm:"not null;default:true"`
	EmailEnabled bool      `json:"email_enabled" gorm:"not null;default:true"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NotificationPreferenceRequest sets one or both switches; a missing field is left as it is.
type NotificationPreferenceRequest struct {
	PushEnabled  *bool `json:"push_enabled"`
	EmailEnabled *bool `json:"email_enabled"`
}

type NotificationPreferenceResponse struct {
	PushEnabled  bool `json:"push_enabled"`
	EmailEnabled bool `json:"email_enabled"`
}
