package services

import (
	"context"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
)

type AssignmentService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AssignmentResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AssignmentResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AssignmentResponse, int64, error)
	GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.AssignmentResponse, int64, error)
	GetActiveByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.AssignmentResponse, error)
	Create(ctx context.Context, req *model.CreateAssignmentRequest) (*model.AssignmentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateAssignmentRequest) (*model.AssignmentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type assignmentServiceImpl struct {
	repo repository.AssignmentRepository
}

func NewAssignmentService(repo repository.AssignmentRepository) AssignmentService {
	return &assignmentServiceImpl{repo: repo}
}

func (s *assignmentServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AssignmentResponse, int64, error) {
	assignments, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = toAssignmentResponse(&assignment)
	}

	return responses, total, nil
}

func (s *assignmentServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.AssignmentResponse, error) {
	assignment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toAssignmentResponse(assignment)
	return &response, nil
}

func (s *assignmentServiceImpl) GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AssignmentResponse, int64, error) {
	assignments, total, err := s.repo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = toAssignmentResponse(&assignment)
	}

	return responses, total, nil
}

func (s *assignmentServiceImpl) GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.AssignmentResponse, int64, error) {
	assignments, total, err := s.repo.FindByLocationID(ctx, locationID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = toAssignmentResponse(&assignment)
	}

	return responses, total, nil
}

func (s *assignmentServiceImpl) GetActiveByPersonnelID(ctx context.Context, personnelID uuid.UUID) (*model.AssignmentResponse, error) {
	assignment, err := s.repo.FindActiveByPersonnelID(ctx, personnelID)
	if err != nil {
		return nil, err
	}

	response := toAssignmentResponse(assignment)
	return &response, nil
}

func (s *assignmentServiceImpl) Create(ctx context.Context, req *model.CreateAssignmentRequest) (*model.AssignmentResponse, error) {
	// Parse start date
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errors.New("invalid start_date format, expected YYYY-MM-DD")
	}

	// Parse end date if provided
	var endDate *time.Time
	if req.EndDate != nil && *req.EndDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			return nil, errors.New("invalid end_date format, expected YYYY-MM-DD")
		}
		endDate = &parsed
	}

	// Validate date range
	if endDate != nil && endDate.Before(startDate) {
		return nil, errors.New("end_date cannot be before start_date")
	}

	// Check for overlapping active assignments for the same personnel
	existingAssignment, err := s.repo.FindActiveByPersonnelID(ctx, req.PersonnelID)
	if err == nil && existingAssignment != nil {
		// Check if dates overlap
		overlap := false
		if endDate == nil {
			// New assignment has no end date (ongoing)
			overlap = true
		} else if existingAssignment.EndDate == nil {
			// Existing assignment has no end date (ongoing)
			overlap = true
		} else {
			// Both have end dates, check for overlap
			if startDate.Before(*existingAssignment.EndDate) && (*endDate).After(existingAssignment.StartDate) {
				overlap = true
			}
		}

		if overlap {
			return nil, errors.New("personnel already has an active overlapping assignment")
		}
	}

	// Set default status if not provided
	if req.Status == "" {
		req.Status = "active"
	}

	assignment := &model.Assignment{
		PersonnelID: req.PersonnelID,
		LocationID:  req.LocationID,
		ShiftID:     req.ShiftID,
		StartDate:   startDate,
		EndDate:     endDate,
		Status:      req.Status,
		Notes:       req.Notes,
		CreatedBy:   req.CreatedBy,
	}

	if err := s.repo.Create(ctx, assignment); err != nil {
		return nil, err
	}

	// Fetch created assignment with relations
	created, err := s.repo.FindByID(ctx, assignment.ID)
	if err != nil {
		return nil, err
	}

	response := toAssignmentResponse(created)
	return &response, nil
}

func (s *assignmentServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateAssignmentRequest) (*model.AssignmentResponse, error) {
	// Find existing assignment
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
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
		existing.EndDate = &endDate
	}

	// Validate date range
	if existing.EndDate != nil && existing.EndDate.Before(existing.StartDate) {
		return nil, errors.New("end_date cannot be before start_date")
	}

	// Update fields
	if req.LocationID != nil {
		existing.LocationID = *req.LocationID
	}
	if req.ShiftID != nil {
		existing.ShiftID = req.ShiftID
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Notes != nil {
		existing.Notes = req.Notes
	}

	// Clear preloaded relations so GORM Save doesn't restore the foreign key
	// fields from the cached association objects (e.g. switching location_id
	// would silently revert because existing.Location.ID still pointed at the
	// previous location).
	existing.Personnel = nil
	existing.Location = nil
	existing.Shift = nil
	existing.Creator = nil

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	// Fetch updated assignment with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toAssignmentResponse(updated)
	return &response, nil
}

func (s *assignmentServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Helper functions

func toAssignmentResponse(assignment *model.Assignment) model.AssignmentResponse {
	response := model.AssignmentResponse{
		ID:          assignment.ID,
		PersonnelID: assignment.PersonnelID,
		LocationID:  assignment.LocationID,
		ShiftID:     assignment.ShiftID,
		StartDate:   assignment.StartDate.Format("2006-01-02"),
		Status:      assignment.Status,
		Notes:       assignment.Notes,
		CreatedBy:   assignment.CreatedBy,
		CreatedAt:   assignment.CreatedAt,
		UpdatedAt:   assignment.UpdatedAt,
	}

	if assignment.EndDate != nil {
		endDate := assignment.EndDate.Format("2006-01-02")
		response.EndDate = &endDate
	}

	if assignment.Personnel != nil {
		personnelResponse := ToPersonnelResponse(assignment.Personnel)
		response.Personnel = &personnelResponse
	}

	if assignment.Location != nil {
		locationResponse := model.LocationResponse{
			ID:               assignment.Location.ID,
			BujpID:           assignment.Location.BujpID,
			Code:             assignment.Location.Code,
			Name:             assignment.Location.Name,
			Address:          assignment.Location.Address,
			Latitude:         assignment.Location.Latitude,
			Longitude:        assignment.Location.Longitude,
			AttendanceRadius: assignment.Location.AttendanceRadius,
			GuardsNeeded:     assignment.Location.GuardsNeeded,
			PicName:          assignment.Location.PicName,
			PicPhone:         assignment.Location.PicPhone,
			Status:           assignment.Location.Status,
			CreatedAt:        assignment.Location.CreatedAt,
			UpdatedAt:        assignment.Location.UpdatedAt,
		}
		response.Location = &locationResponse
	}

	if assignment.Shift != nil {
		shiftResponse := model.ShiftResponse{
			ID:            assignment.Shift.ID,
			BujpID:        assignment.Shift.BujpID,
			Name:          assignment.Shift.Name,
			Description:   assignment.Shift.Description,
			StartTime:     assignment.Shift.StartTime,
			EndTime:       assignment.Shift.EndTime,
			DurationHours: assignment.Shift.DurationHours,
			Status:        assignment.Shift.Status,
			CreatedAt:     assignment.Shift.CreatedAt,
			UpdatedAt:     assignment.Shift.UpdatedAt,
		}
		response.Shift = &shiftResponse
	}

	if assignment.Creator != nil {
		creatorResponse := model.UserResponse{
			ID:        assignment.Creator.ID,
			BujpID:    assignment.Creator.BujpID,
			Email:     assignment.Creator.Email,
			Role:      assignment.Creator.Role,
			FullName:  assignment.Creator.FullName,
			Phone:     assignment.Creator.Phone,
			Photo:     assignment.Creator.Photo,
			Status:    assignment.Creator.Status,
			LastLogin: assignment.Creator.LastLogin,
			CreatedAt: assignment.Creator.CreatedAt,
			UpdatedAt: assignment.Creator.UpdatedAt,
		}
		response.Creator = &creatorResponse
	}

	return response
}
