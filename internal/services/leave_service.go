package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

// presignLeaveDoc fills SupportingDocumentURL from the stored object key.
func presignLeaveDoc(ctx context.Context, r *model.LeaveResponse) {
	if r == nil || r.SupportingDocument == nil || *r.SupportingDocument == "" {
		return
	}
	if url, err := utils.GeneratePresignedURL(ctx, *r.SupportingDocument, time.Hour); err == nil && url != "" {
		u := url
		r.SupportingDocumentURL = &u
	}
}

type LeaveService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LeaveResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.LeaveResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.LeaveResponse, int64, error)
	GetPending(ctx context.Context, page, limit int) ([]model.LeaveResponse, int64, error)
	Create(ctx context.Context, req *model.CreateLeaveRequest) (*model.LeaveResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateLeaveRequest) (*model.LeaveResponse, error)
	Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) (*model.LeaveResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type leaveServiceImpl struct {
	repo repository.LeaveRepository
}

func NewLeaveService(repo repository.LeaveRepository) LeaveService {
	return &leaveServiceImpl{repo: repo}
}

func (s *leaveServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LeaveResponse, int64, error) {
	leaves, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.LeaveResponse, len(leaves))
	for i, leave := range leaves {
		responses[i] = toLeaveResponse(&leave)
		presignLeaveDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *leaveServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.LeaveResponse, error) {
	leave, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toLeaveResponse(leave)
	presignLeaveDoc(ctx, &response)
	return &response, nil
}

func (s *leaveServiceImpl) GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.LeaveResponse, int64, error) {
	leaves, total, err := s.repo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.LeaveResponse, len(leaves))
	for i, leave := range leaves {
		responses[i] = toLeaveResponse(&leave)
		presignLeaveDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *leaveServiceImpl) GetPending(ctx context.Context, page, limit int) ([]model.LeaveResponse, int64, error) {
	leaves, total, err := s.repo.FindPending(ctx, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.LeaveResponse, len(leaves))
	for i, leave := range leaves {
		responses[i] = toLeaveResponse(&leave)
		presignLeaveDoc(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *leaveServiceImpl) Create(ctx context.Context, req *model.CreateLeaveRequest) (*model.LeaveResponse, error) {
	// Parse start date
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errors.New("invalid start_date format, expected YYYY-MM-DD")
	}

	// Parse end date
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, errors.New("invalid end_date format, expected YYYY-MM-DD")
	}

	// Validate date range
	if endDate.Before(startDate) {
		return nil, errors.New("end_date cannot be before start_date")
	}

	// Calculate total days (inclusive)
	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1

	// Check for overlapping leaves for the same personnel. Only ACTIVE
	// requests (pending / approved) block — rejected/cancelled ones are
	// ignored so the user can resubmit after a rejection.
	overlapping, err := s.repo.FindByDateRange(ctx, startDate, endDate, req.PersonnelID)
	if err == nil && len(overlapping) > 0 {
		for _, leave := range overlapping {
			if leave.Status != "approved" && leave.Status != "pending" {
				continue
			}
			statusLabel := "menunggu persetujuan"
			if leave.Status == "approved" {
				statusLabel = "sudah disetujui"
			}
			return nil, fmt.Errorf(
				"sudah ada pengajuan cuti/izin (%s) untuk periode %s s/d %s yang bertabrakan dengan tanggal ini",
				statusLabel,
				leave.StartDate.Format("02 Jan 2006"),
				leave.EndDate.Format("02 Jan 2006"),
			)
		}
	}

	// Override total_days from request if provided, otherwise use calculated
	if req.TotalDays != nil {
		totalDays = *req.TotalDays
	}

	leave := &model.Leave{
		PersonnelID:        req.PersonnelID,
		Type:               req.Type,
		StartDate:          startDate,
		EndDate:            endDate,
		TotalDays:          &totalDays,
		Reason:             req.Reason,
		SupportingDocument: req.SupportingDocument,
		Status:             "pending", // Default status
	}

	if err := s.repo.Create(ctx, leave); err != nil {
		return nil, err
	}

	// Fetch created leave with relations
	created, err := s.repo.FindByID(ctx, leave.ID)
	if err != nil {
		return nil, err
	}

	response := toLeaveResponse(created)
	presignLeaveDoc(ctx, &response)
	return &response, nil
}

func (s *leaveServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateLeaveRequest) (*model.LeaveResponse, error) {
	// Find existing leave
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only allow update if status is pending
	if existing.Status != "pending" {
		return nil, errors.New("can only update pending leave requests")
	}

	// Parse start date if provided
	if req.StartDate != nil && *req.StartDate != "" {
		startDate, err := time.Parse("2006-01-02", *req.StartDate)
		if err != nil {
			return nil, errors.New("invalid start_date format, expected YYYY-MM-DD")
		}
		existing.StartDate = startDate
	}

	// Parse end date if provided
	if req.EndDate != nil && *req.EndDate != "" {
		endDate, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			return nil, errors.New("invalid end_date format, expected YYYY-MM-DD")
		}
		existing.EndDate = endDate
	}

	// Validate date range
	if existing.EndDate.Before(existing.StartDate) {
		return nil, errors.New("end_date cannot be before start_date")
	}

	// Recalculate total days
	totalDays := int(existing.EndDate.Sub(existing.StartDate).Hours()/24) + 1
	existing.TotalDays = &totalDays

	// Override with provided total_days if specified
	if req.TotalDays != nil {
		existing.TotalDays = req.TotalDays
	}

	// Check for overlapping leaves (excluding current leave)
	overlapping, err := s.repo.FindByDateRange(ctx, existing.StartDate, existing.EndDate, existing.PersonnelID)
	if err == nil && len(overlapping) > 0 {
		for _, leave := range overlapping {
			// Skip current leave
			if leave.ID == id {
				continue
			}
			if leave.Status == "approved" || leave.Status == "pending" {
				return nil, errors.New("overlapping leave request already exists for this personnel")
			}
		}
	}

	// Update fields
	if req.Type != nil {
		existing.Type = *req.Type
	}
	if req.Reason != nil {
		existing.Reason = *req.Reason
	}
	if req.SupportingDocument != nil {
		existing.SupportingDocument = req.SupportingDocument
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	// Fetch updated leave with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toLeaveResponse(updated)
	presignLeaveDoc(ctx, &response)
	return &response, nil
}

func (s *leaveServiceImpl) Approve(ctx context.Context, id uuid.UUID, approverID uuid.UUID, status, notes string) (*model.LeaveResponse, error) {
	// Validate status
	if status != "approved" && status != "rejected" {
		return nil, errors.New("status must be either 'approved' or 'rejected'")
	}

	// Find existing leave
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only allow approval if status is pending
	if existing.Status != "pending" {
		return nil, errors.New("can only approve/reject pending leave requests")
	}

	// Approve/reject the leave
	if err := s.repo.Approve(ctx, id, approverID, status, notes); err != nil {
		return nil, err
	}

	// Fetch updated leave with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toLeaveResponse(updated)
	presignLeaveDoc(ctx, &response)
	return &response, nil
}

func (s *leaveServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// Find existing leave
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Only allow deletion if status is pending or rejected
	if existing.Status == "approved" {
		return errors.New("cannot delete approved leave requests")
	}

	return s.repo.Delete(ctx, id)
}

// Helper functions

func toLeaveResponse(leave *model.Leave) model.LeaveResponse {
	response := model.LeaveResponse{
		ID:                 leave.ID,
		PersonnelID:        leave.PersonnelID,
		Type:               leave.Type,
		StartDate:          leave.StartDate.Format("2006-01-02"),
		EndDate:            leave.EndDate.Format("2006-01-02"),
		TotalDays:          leave.TotalDays,
		Reason:             leave.Reason,
		SupportingDocument: leave.SupportingDocument,
		Status:             leave.Status,
		ApproverID:         leave.ApproverID,
		ApproverNotes:      leave.ApproverNotes,
		CreatedAt:          leave.CreatedAt,
		UpdatedAt:          leave.UpdatedAt,
	}

	if leave.ApprovedAt != nil {
		response.ApprovedAt = leave.ApprovedAt
	}

	if leave.Personnel != nil {
		personnelResponse := ToPersonnelResponse(leave.Personnel)
		response.Personnel = &personnelResponse
	}

	if leave.Approver != nil {
		approverResponse := model.UserResponse{
			ID:        leave.Approver.ID,
			BujpID:    leave.Approver.BujpID,
			Email:     leave.Approver.Email,
			Role:      leave.Approver.Role,
			FullName:  leave.Approver.FullName,
			Phone:     leave.Approver.Phone,
			Photo:     leave.Approver.Photo,
			Status:    leave.Approver.Status,
			LastLogin: leave.Approver.LastLogin,
			CreatedAt: leave.Approver.CreatedAt,
			UpdatedAt: leave.Approver.UpdatedAt,
		}
		response.Approver = &approverResponse
	}

	return response
}
