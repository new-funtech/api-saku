package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShiftService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.ShiftResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.ShiftResponse, error)
	GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.ShiftResponse, error)
	Create(ctx context.Context, req *model.CreateShiftRequest) (*model.ShiftResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateShiftRequest) (*model.ShiftResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type shiftServiceImpl struct {
	repo repository.ShiftRepository
}

func NewShiftService(repo repository.ShiftRepository) ShiftService {
	return &shiftServiceImpl{repo: repo}
}

func (s *shiftServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.ShiftResponse, int64, error) {
	shifts, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get shifts: %w", err)
	}

	responses := make([]model.ShiftResponse, len(shifts))
	for i, shift := range shifts {
		responses[i] = toShiftResponse(&shift)
	}

	return responses, total, nil
}

func (s *shiftServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.ShiftResponse, error) {
	shift, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("shift not found")
		}
		return nil, fmt.Errorf("failed to get shift: %w", err)
	}

	response := toShiftResponse(shift)
	return &response, nil
}

func (s *shiftServiceImpl) GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.ShiftResponse, error) {
	shifts, err := s.repo.FindByBujpID(ctx, bujpID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by bujp: %w", err)
	}

	responses := make([]model.ShiftResponse, len(shifts))
	for i, shift := range shifts {
		responses[i] = toShiftResponse(&shift)
	}

	return responses, nil
}

func (s *shiftServiceImpl) Create(ctx context.Context, req *model.CreateShiftRequest) (*model.ShiftResponse, error) {
	shift := &model.Shift{
		BujpID:        req.BujpID,
		Name:          req.Name,
		Description:   req.Description,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		DurationHours: req.DurationHours,
		Status:        req.Status,
	}

	if shift.Status == "" {
		shift.Status = "active"
	}

	if err := s.repo.Create(ctx, shift); err != nil {
		return nil, fmt.Errorf("failed to create shift: %w", err)
	}

	// Fetch the created shift with relations
	created, _ := s.repo.FindByID(ctx, shift.ID)
	response := toShiftResponse(created)
	return &response, nil
}

func (s *shiftServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateShiftRequest) (*model.ShiftResponse, error) {
	shift, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("shift not found")
		}
		return nil, fmt.Errorf("failed to get shift: %w", err)
	}

	if req.BujpID != nil {
		shift.BujpID = req.BujpID
	}
	if req.Name != nil {
		shift.Name = *req.Name
	}
	if req.Description != nil {
		shift.Description = req.Description
	}
	if req.StartTime != nil {
		shift.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		shift.EndTime = *req.EndTime
	}
	if req.DurationHours != nil {
		shift.DurationHours = req.DurationHours
	}
	if req.Status != nil {
		shift.Status = *req.Status
	}

	if err := s.repo.Update(ctx, shift); err != nil {
		return nil, fmt.Errorf("failed to update shift: %w", err)
	}

	// Fetch the updated shift with relations
	updated, _ := s.repo.FindByID(ctx, id)
	response := toShiftResponse(updated)
	return &response, nil
}

func (s *shiftServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("shift not found")
		}
		return fmt.Errorf("failed to get shift: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}

	return nil
}

func toShiftResponse(shift *model.Shift) model.ShiftResponse {
	response := model.ShiftResponse{
		ID:            shift.ID,
		BujpID:        shift.BujpID,
		Name:          shift.Name,
		Description:   shift.Description,
		StartTime:     shift.StartTime,
		EndTime:       shift.EndTime,
		DurationHours: shift.DurationHours,
		Status:        shift.Status,
		CreatedAt:     shift.CreatedAt,
		UpdatedAt:     shift.UpdatedAt,
	}

	if shift.Bujp != nil {
		bujpResponse := toBujpResponse(shift.Bujp)
		response.Bujp = &bujpResponse
	}

	return response
}
