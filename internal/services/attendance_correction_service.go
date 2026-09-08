package services

import (
	"context"
	"strings"
	"time"

	apperrors "github.com/ganiramadhan/ganipedia/backend/internal/errors"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/notifier"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

func combineDateAndHHMM(date time.Time, hhmm *string) *string {
	if hhmm == nil {
		return nil
	}
	s := strings.TrimSpace(*hhmm)
	if s == "" {
		return nil
	}
	if len(s) == 5 { // HH:MM
		s = s + ":00"
	}
	combined := date.Format("2006-01-02") + "T" + s
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", combined, time.Local); err == nil {
		out := t.Format(time.RFC3339)
		return &out
	}

	original := s
	return &original
}

func presignAttendanceCorrectionDoc(ctx context.Context, r *model.AttendanceCorrectionResponse) {
	if r == nil || r.SupportingDocument == nil || *r.SupportingDocument == "" {
		return
	}
	if url, err := utils.GeneratePresignedURL(ctx, *r.SupportingDocument, time.Hour); err == nil && url != "" {
		u := url
		r.SupportingDocumentURL = &u
	}
}

type AttendanceCorrectionService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrectionResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AttendanceCorrectionResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceCorrectionResponse, int64, error)
	GetPending(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrectionResponse, int64, error)
	Create(ctx context.Context, req *model.CreateAttendanceCorrectionRequest) (*model.AttendanceCorrectionResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateAttendanceCorrectionRequest) (*model.AttendanceCorrectionResponse, error)
	Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) (*model.AttendanceCorrectionResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type attendancecorrectionServiceImpl struct {
	repo     repository.AttendanceCorrectionRepository
	notifier *notifier.Notifier
}

func NewAttendanceCorrectionService(repo repository.AttendanceCorrectionRepository, notif *notifier.Notifier) AttendanceCorrectionService {
	return &attendancecorrectionServiceImpl{repo: repo, notifier: notif}
}

func (s *attendancecorrectionServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrectionResponse, int64, error) {
	corrections, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve attendance corrections")
	}

	responses := make([]model.AttendanceCorrectionResponse, len(corrections))
	for i, correction := range corrections {
		responses[i] = toAttendanceCorrectionResponse(&correction)
		presignAttendanceCorrectionDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendancecorrectionServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.AttendanceCorrectionResponse, error) {
	correction, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.NotFound("Attendance correction not found")
		}
		return nil, apperrors.Internal("Failed to retrieve attendance correction")
	}

	response := toAttendanceCorrectionResponse(correction)
	presignAttendanceCorrectionDoc(ctx, &response)
	return &response, nil
}

func (s *attendancecorrectionServiceImpl) GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceCorrectionResponse, int64, error) {
	corrections, total, err := s.repo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve attendance corrections")
	}

	responses := make([]model.AttendanceCorrectionResponse, len(corrections))
	for i, correction := range corrections {
		responses[i] = toAttendanceCorrectionResponse(&correction)
		presignAttendanceCorrectionDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendancecorrectionServiceImpl) GetPending(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceCorrectionResponse, int64, error) {
	corrections, total, err := s.repo.FindPending(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, apperrors.Internal("Failed to retrieve pending corrections")
	}

	responses := make([]model.AttendanceCorrectionResponse, len(corrections))
	for i, correction := range corrections {
		responses[i] = toAttendanceCorrectionResponse(&correction)
		presignAttendanceCorrectionDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendancecorrectionServiceImpl) Create(ctx context.Context, req *model.CreateAttendanceCorrectionRequest) (*model.AttendanceCorrectionResponse, error) {
	// Parse correction date
	correctionDate, err := utils.ParseDate(req.CorrectionDate)
	if err != nil {
		return nil, apperrors.Validation("Invalid correction date format, expected YYYY-MM-DD")
	}

	// Validate at least one time field is provided based on correction type
	if req.CorrectionType == "checkin" && req.CheckinTime == nil {
		return nil, apperrors.Validation("Check-in time is required for check-in correction")
	}
	if req.CorrectionType == "checkout" && req.CheckoutTime == nil {
		return nil, apperrors.Validation("Check-out time is required for check-out correction")
	}
	if req.CorrectionType == "both" && (req.CheckinTime == nil || req.CheckoutTime == nil) {
		return nil, apperrors.Validation("Both check-in and check-out times are required for both correction")
	}

	correction := &model.AttendanceCorrection{
		PersonnelID:        req.PersonnelID,
		CorrectionDate:     correctionDate,
		CorrectionType:     req.CorrectionType,
		CheckinTime:        combineDateAndHHMM(correctionDate, req.CheckinTime),
		CheckoutTime:       combineDateAndHHMM(correctionDate, req.CheckoutTime),
		Reason:             req.Reason,
		SupportingDocument: req.SupportingDocument,
		Status:             "pending", // Default status
	}

	if err := s.repo.Create(ctx, correction); err != nil {
		return nil, apperrors.Internal("Failed to create attendance correction")
	}

	created, err := s.repo.FindByID(ctx, correction.ID)
	if err != nil {
		return nil, apperrors.Internal("Failed to retrieve created correction")
	}

	response := toAttendanceCorrectionResponse(created)
	presignAttendanceCorrectionDoc(ctx, &response)
	return &response, nil
}

func (s *attendancecorrectionServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateAttendanceCorrectionRequest) (*model.AttendanceCorrectionResponse, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.NotFound("Attendance correction not found")
		}
		return nil, apperrors.Internal("Failed to retrieve attendance correction")
	}

	if existing.Status != "pending" {
		return nil, apperrors.BadRequest("Can only update pending correction requests")
	}

	if req.CorrectionDate != nil {
		correctionDate, err := utils.ParseDate(*req.CorrectionDate)
		if err != nil {
			return nil, apperrors.Validation("Invalid correction date format, expected YYYY-MM-DD")
		}
		existing.CorrectionDate = correctionDate
	}
	if req.CorrectionType != nil {
		existing.CorrectionType = *req.CorrectionType
	}
	if req.CheckinTime != nil {
		existing.CheckinTime = combineDateAndHHMM(existing.CorrectionDate, req.CheckinTime)
	}
	if req.CheckoutTime != nil {
		existing.CheckoutTime = combineDateAndHHMM(existing.CorrectionDate, req.CheckoutTime)
	}
	if req.Reason != nil {
		existing.Reason = *req.Reason
	}
	if req.SupportingDocument != nil {
		existing.SupportingDocument = req.SupportingDocument
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, apperrors.Internal("Failed to update attendance correction")
	}

	// Fetch updated correction with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("Failed to retrieve updated correction")
	}

	response := toAttendanceCorrectionResponse(updated)
	presignAttendanceCorrectionDoc(ctx, &response)
	return &response, nil
}

func (s *attendancecorrectionServiceImpl) Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) (*model.AttendanceCorrectionResponse, error) {
	// Validate status
	if status != "approved" && status != "rejected" {
		return nil, apperrors.Validation("Status must be either 'approved' or 'rejected'")
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.NotFound("Attendance correction not found")
		}
		return nil, apperrors.Internal("Failed to retrieve attendance correction")
	}

	if existing.Status != "pending" {
		return nil, apperrors.BadRequest("Can only approve/reject pending correction requests")
	}

	if err := s.repo.Approve(ctx, id, approverID, status, notes); err != nil {
		return nil, apperrors.Internal("Failed to approve/reject attendance correction")
	}

	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("Failed to retrieve updated correction")
	}

	response := toAttendanceCorrectionResponse(updated)
	presignAttendanceCorrectionDoc(ctx, &response)
	s.notifier.CorrectionDecision(ctx, updated, status)
	return &response, nil
}

func (s *attendancecorrectionServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return apperrors.NotFound("Attendance correction not found")
		}
		return apperrors.Internal("Failed to retrieve attendance correction")
	}

	if existing.Status == "approved" {
		return apperrors.BadRequest("Cannot delete approved correction requests")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return apperrors.Internal("Failed to delete attendance correction")
	}

	return nil
}

func formatStoredHHMM(v *string) *string {
	if v == nil || *v == "" {
		return v
	}
	s := strings.TrimSpace(*v)
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			out := t.Format("15:04")
			return &out
		}
	}
	return v
}

func toAttendanceCorrectionResponse(correction *model.AttendanceCorrection) model.AttendanceCorrectionResponse {
	response := model.AttendanceCorrectionResponse{
		ID:                 correction.ID,
		PersonnelID:        correction.PersonnelID,
		CorrectionDate:     correction.CorrectionDate.Format("2006-01-02"),
		CorrectionType:     correction.CorrectionType,
		CheckinTime:        formatStoredHHMM(correction.CheckinTime),
		CheckoutTime:       formatStoredHHMM(correction.CheckoutTime),
		Reason:             correction.Reason,
		SupportingDocument: correction.SupportingDocument,
		Status:             correction.Status,
		ApproverID:         correction.ApproverID,
		ApprovedAt:         correction.ApprovedAt,
		ApproverNotes:      correction.ApproverNotes,
		CreatedAt:          correction.CreatedAt,
		UpdatedAt:          correction.UpdatedAt,
	}

	if correction.Personnel != nil {
		personnelResponse := ToPersonnelResponse(correction.Personnel)
		response.Personnel = &personnelResponse
	}

	if correction.Approver != nil {
		approverResponse := model.UserResponse{
			ID:        correction.Approver.ID,
			BujpID:    correction.Approver.BujpID,
			Email:     correction.Approver.Email,
			Role:      correction.Approver.Role,
			FullName:  correction.Approver.FullName,
			Phone:     correction.Approver.Phone,
			Photo:     correction.Approver.Photo,
			Status:    correction.Approver.Status,
			LastLogin: correction.Approver.LastLogin,
			CreatedAt: correction.Approver.CreatedAt,
			UpdatedAt: correction.Approver.UpdatedAt,
		}
		response.Approver = &approverResponse
	}

	return response
}
