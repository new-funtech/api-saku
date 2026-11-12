package services

import (
	"context"
	"fmt"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/realtime"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationService interface {
	// Outward-facing — used by NotificationHandler.
	List(ctx context.Context, userID uuid.UUID, page, limit int) ([]model.NotificationResponse, int64, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
	ClearRead(ctx context.Context, userID uuid.UUID) error

	NotifyLoanSubmitted(ctx context.Context, tx *gorm.DB, loan *model.Loan) ([]model.Notification, error)
	NotifyLoanApprovalDecision(ctx context.Context, tx *gorm.DB, loan *model.Loan, approval *model.LoanApproval, approved bool) ([]model.Notification, error)
	NotifyLoanCancelled(ctx context.Context, tx *gorm.DB, loan *model.Loan) ([]model.Notification, error)

	PushRealtime(rows []model.Notification)

	NotifyOvertimeSubmitted(ctx context.Context, o *model.Overtime, applicantName string)
	NotifyOvertimeDecision(ctx context.Context, o *model.Overtime, approved bool)
}

type notificationServiceImpl struct {
	repo          repository.NotificationRepository
	personnelRepo repository.PersonnelRepository
	notifier      *notifier.Notifier
	hub           *realtime.Hub
}

func NewNotificationService(
	repo repository.NotificationRepository,
	personnelRepo repository.PersonnelRepository,
	notif *notifier.Notifier,
	hub *realtime.Hub,
) NotificationService {
	return &notificationServiceImpl{repo: repo, personnelRepo: personnelRepo, notifier: notif, hub: hub}
}

// === Outward-facing ===

func (s *notificationServiceImpl) List(ctx context.Context, userID uuid.UUID, page, limit int) ([]model.NotificationResponse, int64, error) {
	items, total, err := s.repo.FindByRecipient(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	return s.toResponses(ctx, items), total, nil
}

// toResponses converts rows and attaches each loan's number with a single lookup.
func (s *notificationServiceImpl) toResponses(ctx context.Context, items []model.Notification) []model.NotificationResponse {
	out := make([]model.NotificationResponse, len(items))
	var loanIDs []uuid.UUID
	for i := range items {
		out[i] = model.ToNotificationResponse(&items[i])
		if items[i].EntityType == "loan" {
			loanIDs = append(loanIDs, items[i].EntityID)
		}
	}
	if numbers, err := s.repo.LoanNumbers(ctx, loanIDs); err == nil {
		for i := range out {
			out[i].LoanNumber = numbers[out[i].EntityID]
		}
	}
	return out
}

func (s *notificationServiceImpl) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *notificationServiceImpl) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *notificationServiceImpl) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *notificationServiceImpl) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *notificationServiceImpl) ClearRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.ClearRead(ctx, userID)
}

// === Loan workflow integration ===

func (s *notificationServiceImpl) NotifyLoanSubmitted(ctx context.Context, tx *gorm.DB, loan *model.Loan) ([]model.Notification, error) {
	if loan == nil {
		return nil, nil
	}
	applicantName := s.applicantName(ctx, loan)
	eventID := fmt.Sprintf("loan_submitted:%s", loan.ID)
	var created []model.Notification

	if applicantID, ok := s.applicantUserID(ctx, loan); ok {
		title, body := notifLoanSubmittedApplicant()
		rows, err := s.createForRecipients(ctx, tx, []uuid.UUID{applicantID}, model.NotificationTypeLoanSubmitted, title, body, loan, eventID)
		if err != nil {
			return nil, err
		}
		created = append(created, rows...)
	}

	reviewers := s.recipientsForLevel(ctx, model.LoanApprovalLevelBujp, loan.BujpID)
	if len(reviewers) > 0 {
		title, body := notifLoanSubmittedApprover(applicantName)
		rows, err := s.createForRecipients(ctx, tx, userIDs(reviewers), model.NotificationTypeLoanReviewRequired, title, body, loan, eventID)
		if err != nil {
			return nil, err
		}
		created = append(created, rows...)
	}
	return created, nil
}

func (s *notificationServiceImpl) NotifyLoanApprovalDecision(ctx context.Context, tx *gorm.DB, loan *model.Loan, approval *model.LoanApproval, approved bool) ([]model.Notification, error) {
	if loan == nil || approval == nil {
		return nil, nil
	}
	eventID := fmt.Sprintf("loan_approval:%s:%s", approval.ID, approval.Status)
	var created []model.Notification

	if !approved {
		if applicantID, ok := s.applicantUserID(ctx, loan); ok {
			title, body := notifLoanRejected(approval.Notes)
			rows, err := s.createForRecipients(ctx, tx, []uuid.UUID{applicantID}, model.NotificationTypeLoanRejected, title, body, loan, eventID)
			if err != nil {
				return nil, err
			}
			created = append(created, rows...)
		}
		return created, nil
	}

	isFinalLevel := approval.ApprovalLevel == model.LoanApprovalLevelBprks
	if applicantID, ok := s.applicantUserID(ctx, loan); ok {
		var title, body, notifType string
		if isFinalLevel {
			title, body = notifLoanCompleted()
			notifType = model.NotificationTypeLoanCompleted
		} else {
			title, body = notifLoanApproved(approval.ApprovalLevelName)
			notifType = model.NotificationTypeLoanApproved
		}
		rows, err := s.createForRecipients(ctx, tx, []uuid.UUID{applicantID}, notifType, title, body, loan, eventID)
		if err != nil {
			return nil, err
		}
		created = append(created, rows...)
	}

	if !isFinalLevel {
		nextLevel := approval.ApprovalLevel + 1
		nextReviewers := s.recipientsForLevel(ctx, nextLevel, loan.BujpID)
		if len(nextReviewers) > 0 {
			title, body := notifLoanReviewRequired(s.applicantName(ctx, loan))
			rows, err := s.createForRecipients(ctx, tx, userIDs(nextReviewers), model.NotificationTypeLoanReviewRequired, title, body, loan, eventID)
			if err != nil {
				return nil, err
			}
			created = append(created, rows...)
		}
	}
	return created, nil
}

func (s *notificationServiceImpl) NotifyLoanCancelled(ctx context.Context, tx *gorm.DB, loan *model.Loan) ([]model.Notification, error) {
	if loan == nil {
		return nil, nil
	}
	eventID := fmt.Sprintf("loan_cancelled:%s", loan.ID)
	var recipients []uuid.UUID
	if applicantID, ok := s.applicantUserID(ctx, loan); ok {
		recipients = append(recipients, applicantID)
	}
	reviewers := s.recipientsForLevel(ctx, model.LoanApprovalLevelBujp, loan.BujpID)
	recipients = append(recipients, userIDs(reviewers)...)
	if len(recipients) == 0 {
		return nil, nil
	}
	title, body := notifLoanCancelled()
	return s.createForRecipients(ctx, tx, recipients, model.NotificationTypeLoanCancelled, title, body, loan, eventID)
}

func (s *notificationServiceImpl) PushRealtime(rows []model.Notification) {
	if s.hub == nil {
		return
	}
	if len(rows) == 0 {
		return
	}
	responses := s.toResponses(context.Background(), rows)
	for i := range rows {
		s.hub.SendToUser(rows[i].RecipientUserID, "notification.created", responses[i])
	}
}

// === helpers ===

func (s *notificationServiceImpl) applicantUserID(ctx context.Context, loan *model.Loan) (uuid.UUID, bool) {
	if s.personnelRepo == nil {
		return uuid.Nil, false
	}
	personnel, err := s.personnelRepo.FindByID(ctx, loan.PersonnelID)
	if err != nil || personnel == nil || personnel.UserID == nil || *personnel.UserID == uuid.Nil {
		return uuid.Nil, false
	}
	return *personnel.UserID, true
}

func (s *notificationServiceImpl) applicantName(ctx context.Context, loan *model.Loan) string {
	if loan.Personnel != nil && loan.Personnel.FullName != "" {
		return loan.Personnel.FullName
	}
	if s.personnelRepo != nil {
		if personnel, err := s.personnelRepo.FindByID(ctx, loan.PersonnelID); err == nil && personnel != nil {
			return personnel.FullName
		}
	}
	return "seorang karyawan"
}

func (s *notificationServiceImpl) recipientsForLevel(ctx context.Context, level int, bujpID *uuid.UUID) []model.User {
	if s.notifier == nil {
		return nil
	}
	switch level {
	case model.LoanApprovalLevelBujp:
		return s.notifier.AdminCompanyUsers(ctx, bujpID)
	case model.LoanApprovalLevelPusat:
		return s.notifier.AdminPusatUsers(ctx)
	case model.LoanApprovalLevelBprks:
		return s.notifier.AdminBprksUsers(ctx)
	default:
		return nil
	}
}

func userIDs(users []model.User) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		out = append(out, u.ID)
	}
	return out
}

func (s *notificationServiceImpl) createForRecipients(
	ctx context.Context,
	tx *gorm.DB,
	recipientIDs []uuid.UUID,
	notifType, title, body string,
	loan *model.Loan,
	eventID string,
) ([]model.Notification, error) {
	return s.createFor(ctx, tx, recipientIDs, loan.BujpID, model.NotificationEntityLoan, loan.ID, notifType, title, body, eventID)
}

// createFor stores one notification per recipient about any entity. The event id makes it
// idempotent: a repeated event is not delivered twice.
func (s *notificationServiceImpl) createFor(
	ctx context.Context,
	tx *gorm.DB,
	recipientIDs []uuid.UUID,
	bujpID *uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	notifType, title, body, eventID string,
) ([]model.Notification, error) {
	repo := s.repo
	if tx != nil {
		repo = s.repo.WithTx(tx)
	}
	created := make([]model.Notification, 0, len(recipientIDs))
	for _, recipientID := range recipientIDs {
		if recipientID == uuid.Nil {
			continue
		}
		n := &model.Notification{
			RecipientUserID: recipientID,
			BujpID:          bujpID,
			Type:            notifType,
			Title:           title,
			Body:            body,
			EntityType:      entityType,
			EntityID:        entityID,
			EventID:         eventID,
		}
		ok, err := repo.Create(ctx, n)
		if err != nil {
			return nil, err
		}
		if ok {
			created = append(created, *n)
		}
	}
	return created, nil
}

// NotifyOvertimeSubmitted tells the company admins a request awaits their decision.
func (s *notificationServiceImpl) NotifyOvertimeSubmitted(ctx context.Context, o *model.Overtime, applicantName string) {
	if o == nil || o.Personnel == nil || s.notifier == nil {
		return
	}
	bujpID := o.Personnel.BujpID
	reviewers := s.notifier.AdminCompanyUsers(ctx, &bujpID)
	title, body := notifOvertimeReviewRequired(applicantName, o.Date, o.TotalHours)
	rows, err := s.createFor(ctx, nil, userIDs(reviewers), &bujpID, model.NotificationEntityOvertime, o.ID,
		model.NotificationTypeOvertimeReviewRequired, title, body, fmt.Sprintf("overtime_submitted:%s", o.ID))
	if err == nil {
		s.PushRealtime(rows)
	}
}

// NotifyOvertimeDecision tells the applicant how the request was decided.
func (s *notificationServiceImpl) NotifyOvertimeDecision(ctx context.Context, o *model.Overtime, approved bool) {
	if o == nil || o.Personnel == nil || o.Personnel.UserID == nil {
		return
	}
	bujpID := o.Personnel.BujpID
	notifType := model.NotificationTypeOvertimeApproved
	if !approved {
		notifType = model.NotificationTypeOvertimeRejected
	}
	title, body := notifOvertimeDecision(approved, o.Date, o.ApproverNotes)
	rows, err := s.createFor(ctx, nil, []uuid.UUID{*o.Personnel.UserID}, &bujpID, model.NotificationEntityOvertime, o.ID,
		notifType, title, body, fmt.Sprintf("overtime_decision:%s:%s", o.ID, o.Status))
	if err == nil {
		s.PushRealtime(rows)
	}
}
