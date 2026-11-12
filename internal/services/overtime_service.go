package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
	"github.com/google/uuid"
)

const (
	overtimeMinHours      = 0.5
	overtimeMaxHours      = 12.0
	overtimePastDays      = 31 // how far back a request may be filed
	overtimeFutureDays    = 7  // how far ahead
	overtimeMinReasonSize = 5
)

type OvertimeService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.OvertimeResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.OvertimeResponse, error)
	Create(ctx context.Context, req *model.CreateOvertimeRequest) (*model.OvertimeResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateOvertimeRequest) (*model.OvertimeResponse, error)
	Decide(ctx context.Context, id, approverID uuid.UUID, status, notes string) (*model.OvertimeResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type overtimeServiceImpl struct {
	repo          repository.OvertimeRepository
	notifier      *notifier.Notifier
	notifications NotificationService
}

func NewOvertimeService(repo repository.OvertimeRepository, notif *notifier.Notifier, notifications NotificationService) OvertimeService {
	return &overtimeServiceImpl{repo: repo, notifier: notif, notifications: notifications}
}

// parseClock reads "HH:MM" into minutes since midnight.
func parseClock(v string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid time %q, expected HH:MM", v)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// validateOvertime checks the shape of a request and returns its normalized date and hours.
func validateOvertime(date, start, end, reason string, now time.Time) (time.Time, float64, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, 0, errors.New("invalid date, expected YYYY-MM-DD")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if day.Before(today.AddDate(0, 0, -overtimePastDays)) {
		return time.Time{}, 0, fmt.Errorf("lembur hanya dapat diajukan paling lama %d hari ke belakang", overtimePastDays)
	}
	if day.After(today.AddDate(0, 0, overtimeFutureDays)) {
		return time.Time{}, 0, fmt.Errorf("lembur hanya dapat diajukan paling jauh %d hari ke depan", overtimeFutureDays)
	}
	s, err := parseClock(start)
	if err != nil {
		return time.Time{}, 0, err
	}
	e, err := parseClock(end)
	if err != nil {
		return time.Time{}, 0, err
	}
	if e <= s {
		return time.Time{}, 0, errors.New("jam selesai harus setelah jam mulai")
	}
	hours := float64(e-s) / 60
	if hours < overtimeMinHours {
		return time.Time{}, 0, errors.New("durasi lembur minimal 30 menit")
	}
	if hours > overtimeMaxHours {
		return time.Time{}, 0, fmt.Errorf("durasi lembur maksimal %.0f jam", overtimeMaxHours)
	}
	if len([]rune(strings.TrimSpace(reason))) < overtimeMinReasonSize {
		return time.Time{}, 0, fmt.Errorf("alasan lembur minimal %d karakter", overtimeMinReasonSize)
	}
	return day, hours, nil
}

func toOvertimeResponse(o *model.Overtime) model.OvertimeResponse {
	r := model.OvertimeResponse{
		ID: o.ID, PersonnelID: o.PersonnelID, Date: o.Date.Format("2006-01-02"),
		StartTime: o.StartTime, EndTime: o.EndTime, TotalHours: o.TotalHours, Reason: o.Reason,
		Status: o.Status, ApproverID: o.ApproverID, ApprovedAt: o.ApprovedAt, ApproverNotes: o.ApproverNotes,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
	if o.Personnel != nil {
		p := ToPersonnelResponse(o.Personnel)
		r.Personnel = &p
	}
	if o.Approver != nil {
		u := toUserResponse(*o.Approver)
		r.Approver = &u
	}
	return r
}

func (s *overtimeServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.OvertimeResponse, int64, error) {
	items, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.OvertimeResponse, len(items))
	for i := range items {
		out[i] = toOvertimeResponse(&items[i])
	}
	return out, total, nil
}

func (s *overtimeServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.OvertimeResponse, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("overtime not found")
	}
	r := toOvertimeResponse(o)
	return &r, nil
}

func (s *overtimeServiceImpl) Create(ctx context.Context, req *model.CreateOvertimeRequest) (*model.OvertimeResponse, error) {
	if req.PersonnelID == uuid.Nil {
		return nil, errors.New("personnel_id is required")
	}
	day, hours, err := validateOvertime(req.Date, req.StartTime, req.EndTime, req.Reason, time.Now())
	if err != nil {
		return nil, err
	}
	o := &model.Overtime{
		PersonnelID: req.PersonnelID, Date: day,
		StartTime: strings.TrimSpace(req.StartTime), EndTime: strings.TrimSpace(req.EndTime),
		TotalHours: hours, Reason: strings.TrimSpace(req.Reason), Status: "pending",
	}
	conflict, err := s.repo.CreateIfFree(ctx, o)
	if err != nil {
		return nil, err
	}
	if conflict != nil {
		return nil, fmt.Errorf("sudah ada pengajuan lembur (%s-%s) pada tanggal ini yang bertabrakan dengan jam tersebut",
			conflict.StartTime, conflict.EndTime)
	}
	created, err := s.repo.FindByID(ctx, o.ID)
	if err != nil {
		return nil, errors.New("overtime not found")
	}
	if s.notifications != nil && created.Personnel != nil {
		s.notifications.NotifyOvertimeSubmitted(ctx, created, created.Personnel.FullName)
	}
	r := toOvertimeResponse(created)
	return &r, nil
}

func (s *overtimeServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateOvertimeRequest) (*model.OvertimeResponse, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("overtime not found")
	}
	if o.Status != "pending" {
		return nil, errors.New("can only update pending overtime requests")
	}
	date, start, end, reason := o.Date.Format("2006-01-02"), o.StartTime, o.EndTime, o.Reason
	if req.Date != nil {
		date = *req.Date
	}
	if req.StartTime != nil {
		start = *req.StartTime
	}
	if req.EndTime != nil {
		end = *req.EndTime
	}
	if req.Reason != nil {
		reason = *req.Reason
	}
	day, hours, err := validateOvertime(date, start, end, reason, time.Now())
	if err != nil {
		return nil, err
	}
	o.Date, o.StartTime, o.EndTime, o.TotalHours, o.Reason = day, strings.TrimSpace(start), strings.TrimSpace(end), hours, strings.TrimSpace(reason)
	if err := s.repo.UpdatePending(ctx, o); err != nil {
		if errors.Is(err, repository.ErrNotPending) {
			return nil, errors.New("can only update pending overtime requests")
		}
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *overtimeServiceImpl) Decide(ctx context.Context, id, approverID uuid.UUID, status, notes string) (*model.OvertimeResponse, error) {
	if status != "approved" && status != "rejected" {
		return nil, errors.New("status must be 'approved' or 'rejected'")
	}
	if status == "rejected" && strings.TrimSpace(notes) == "" {
		return nil, errors.New("alasan penolakan is required")
	}
	if err := s.repo.Decide(ctx, id, approverID, status, strings.TrimSpace(notes)); err != nil {
		if errors.Is(err, repository.ErrNotPending) {
			return nil, errors.New("can only approve/reject pending overtime requests")
		}
		return nil, err
	}
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.notifier.OvertimeDecision(ctx, updated, status)
	if s.notifications != nil {
		s.notifications.NotifyOvertimeDecision(ctx, updated, status == "approved")
	}
	r := toOvertimeResponse(updated)
	return &r, nil
}

func (s *overtimeServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteUnapproved(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotPending) {
			return errors.New("approved overtime requests cannot be deleted")
		}
		return err
	}
	return nil
}
