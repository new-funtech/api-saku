package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	NotificationTypeLoanSubmitted      = "LOAN_SUBMITTED"
	NotificationTypeLoanReviewRequired = "LOAN_REVIEW_REQUIRED"
	NotificationTypeLoanApproved       = "LOAN_APPROVED"
	NotificationTypeLoanRejected       = "LOAN_REJECTED"
	NotificationTypeLoanCancelled      = "LOAN_CANCELLED"
	NotificationTypeLoanCompleted      = "LOAN_COMPLETED"
)

const NotificationEntityLoan = "loan"

type Notification struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	RecipientUserID uuid.UUID  `json:"recipient_user_id" gorm:"type:uuid;not null;index"`
	ActorUserID     *uuid.UUID `json:"actor_user_id" gorm:"type:uuid"`
	BujpID          *uuid.UUID `json:"bujp_id" gorm:"type:uuid;index"`
	Type            string     `json:"type" gorm:"type:varchar(50);not null;index"`
	Title           string     `json:"title" gorm:"type:varchar(200);not null"`
	Body            string     `json:"body" gorm:"type:text;not null"`
	EntityType      string     `json:"entity_type" gorm:"type:varchar(50);not null"`
	EntityID        uuid.UUID  `json:"entity_id" gorm:"type:uuid;not null;index"`
	ActionURL       *string    `json:"action_url" gorm:"type:varchar(255)"`
	Metadata        *string    `json:"metadata" gorm:"type:jsonb"`
	EventID         string     `json:"event_id" gorm:"type:varchar(150);not null"`
	IsRead          bool       `json:"is_read" gorm:"not null;default:false;index"`
	ReadAt          *time.Time `json:"read_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

type NotificationResponse struct {
	ID         uuid.UUID  `json:"id"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	EntityType string     `json:"entity_type"`
	EntityID   uuid.UUID  `json:"entity_id"`
	ActionURL  *string    `json:"action_url,omitempty"`
	Metadata   *string    `json:"metadata,omitempty"`
	IsRead     bool       `json:"is_read"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func ToNotificationResponse(n *Notification) NotificationResponse {
	return NotificationResponse{
		ID:         n.ID,
		Type:       n.Type,
		Title:      n.Title,
		Body:       n.Body,
		EntityType: n.EntityType,
		EntityID:   n.EntityID,
		ActionURL:  n.ActionURL,
		Metadata:   n.Metadata,
		IsRead:     n.IsRead,
		ReadAt:     n.ReadAt,
		CreatedAt:  n.CreatedAt,
	}
}
