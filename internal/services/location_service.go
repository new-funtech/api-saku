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

type LocationService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LocationResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.LocationResponse, error)
	GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.LocationResponse, error)
	Create(ctx context.Context, req *model.CreateLocationRequest) (*model.LocationResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateLocationRequest) (*model.LocationResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type locationServiceImpl struct {
	repo repository.LocationRepository
}

func NewLocationService(repo repository.LocationRepository) LocationService {
	return &locationServiceImpl{repo: repo}
}

func (s *locationServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LocationResponse, int64, error) {
	locations, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get locations: %w", err)
	}

	responses := make([]model.LocationResponse, len(locations))
	for i, location := range locations {
		responses[i] = toLocationResponse(&location)
	}

	return responses, total, nil
}

func (s *locationServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.LocationResponse, error) {
	location, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("location not found")
		}
		return nil, fmt.Errorf("failed to get location: %w", err)
	}

	response := toLocationResponse(location)
	return &response, nil
}

func (s *locationServiceImpl) GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.LocationResponse, error) {
	locations, err := s.repo.FindByBujpID(ctx, bujpID)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations by bujp: %w", err)
	}

	responses := make([]model.LocationResponse, len(locations))
	for i, location := range locations {
		responses[i] = toLocationResponse(&location)
	}

	return responses, nil
}

func (s *locationServiceImpl) Create(ctx context.Context, req *model.CreateLocationRequest) (*model.LocationResponse, error) {
	// Check if code already exists
	existing, _ := s.repo.FindByCode(ctx, req.Code)
	if existing != nil {
		return nil, fmt.Errorf("location with code %s already exists", req.Code)
	}

	location := &model.Location{
		BujpID:           req.BujpID,
		Code:             req.Code,
		Name:             req.Name,
		Address:          req.Address,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		AttendanceRadius: req.AttendanceRadius,
		GuardsNeeded:     req.GuardsNeeded,
		PicName:          req.PicName,
		PicPhone:         req.PicPhone,
		Status:           req.Status,
	}

	if location.AttendanceRadius == 0 {
		location.AttendanceRadius = 100
	}
	if location.GuardsNeeded == 0 {
		location.GuardsNeeded = 1
	}
	if location.Status == "" {
		location.Status = "active"
	}

	if err := s.repo.Create(ctx, location); err != nil {
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	// Fetch the created location with relations
	created, _ := s.repo.FindByID(ctx, location.ID)
	response := toLocationResponse(created)
	return &response, nil
}

func (s *locationServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateLocationRequest) (*model.LocationResponse, error) {
	location, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("location not found")
		}
		return nil, fmt.Errorf("failed to get location: %w", err)
	}

	// Check if code is being changed and if it already exists
	if req.Code != nil && *req.Code != location.Code {
		existing, _ := s.repo.FindByCode(ctx, *req.Code)
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("location with code %s already exists", *req.Code)
		}
		location.Code = *req.Code
	}

	if req.BujpID != nil {
		location.BujpID = *req.BujpID
	}
	if req.Name != nil {
		location.Name = *req.Name
	}
	if req.Address != nil {
		location.Address = *req.Address
	}
	if req.Latitude != nil {
		location.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		location.Longitude = req.Longitude
	}
	if req.AttendanceRadius != nil {
		location.AttendanceRadius = *req.AttendanceRadius
	}
	if req.GuardsNeeded != nil {
		location.GuardsNeeded = *req.GuardsNeeded
	}
	if req.PicName != nil {
		location.PicName = req.PicName
	}
	if req.PicPhone != nil {
		location.PicPhone = req.PicPhone
	}
	if req.Status != nil {
		location.Status = *req.Status
	}

	if err := s.repo.Update(ctx, location); err != nil {
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	// Fetch the updated location with relations
	updated, _ := s.repo.FindByID(ctx, id)
	response := toLocationResponse(updated)
	return &response, nil
}

func (s *locationServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("location not found")
		}
		return fmt.Errorf("failed to get location: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	return nil
}

func toLocationResponse(location *model.Location) model.LocationResponse {
	response := model.LocationResponse{
		ID:               location.ID,
		BujpID:           location.BujpID,
		Code:             location.Code,
		Name:             location.Name,
		Address:          location.Address,
		Latitude:         location.Latitude,
		Longitude:        location.Longitude,
		AttendanceRadius: location.AttendanceRadius,
		GuardsNeeded:     location.GuardsNeeded,
		PicName:          location.PicName,
		PicPhone:         location.PicPhone,
		Status:           location.Status,
		CreatedAt:        location.CreatedAt,
		UpdatedAt:        location.UpdatedAt,
	}

	if location.Bujp != nil {
		bujpResponse := toBujpResponse(location.Bujp)
		response.Bujp = &bujpResponse
	}

	return response
}
