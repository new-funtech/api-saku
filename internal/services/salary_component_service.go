package services

import (
	"context"
	"fmt"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SalaryComponentService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.SalaryComponentResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.SalaryComponentResponse, error)
	Create(ctx context.Context, req *model.CreateSalaryComponentRequest) (*model.SalaryComponentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateSalaryComponentRequest) (*model.SalaryComponentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type salarycomponentServiceImpl struct {
	repo repository.SalaryComponentRepository
}

func NewSalaryComponentService(repo repository.SalaryComponentRepository) SalaryComponentService {
	return &salarycomponentServiceImpl{repo: repo}
}

func (s *salarycomponentServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.SalaryComponentResponse, int64, error) {
	components, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.SalaryComponentResponse, 0, len(components))
	for _, component := range components {
		responses = append(responses, toSalaryComponentResponse(&component))
	}

	return responses, total, nil
}

func (s *salarycomponentServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.SalaryComponentResponse, error) {
	component, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("salary component not found")
		}
		return nil, err
	}

	response := toSalaryComponentResponse(component)
	return &response, nil
}

func (s *salarycomponentServiceImpl) Create(ctx context.Context, req *model.CreateSalaryComponentRequest) (*model.SalaryComponentResponse, error) {
	// Check duplicate code
	if existing, _ := s.repo.FindByCode(ctx, req.Code); existing != nil {
		return nil, fmt.Errorf("salary component code already exists")
	}

	// Set defaults
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	component := &model.SalaryComponent{
		BujpID:            req.BujpID,
		Code:              req.Code,
		Name:              req.Name,
		Type:              req.Type,
		CalculationMethod: req.CalculationMethod,
		Amount:            req.Amount,
		IsActive:          isActive,
		Description:       req.Description,
	}

	if err := s.repo.Create(ctx, component); err != nil {
		return nil, err
	}

	// Reload to get relations
	created, err := s.repo.FindByID(ctx, component.ID)
	if err != nil {
		return nil, err
	}

	response := toSalaryComponentResponse(created)
	return &response, nil
}

func (s *salarycomponentServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSalaryComponentRequest) (*model.SalaryComponentResponse, error) {
	component, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("salary component not found")
		}
		return nil, err
	}

	// Check duplicate code if changed
	if req.Code != nil && *req.Code != component.Code {
		if existing, _ := s.repo.FindByCode(ctx, *req.Code); existing != nil && existing.ID != id {
			return nil, fmt.Errorf("salary component code already exists")
		}
		component.Code = *req.Code
	}

	// Update fields
	if req.BujpID != nil {
		component.BujpID = req.BujpID
	}
	if req.Name != nil {
		component.Name = *req.Name
	}
	if req.Type != nil {
		component.Type = *req.Type
	}
	if req.CalculationMethod != nil {
		component.CalculationMethod = *req.CalculationMethod
	}
	if req.Amount != nil {
		component.Amount = *req.Amount
	}
	if req.IsActive != nil {
		component.IsActive = *req.IsActive
	}
	if req.Description != nil {
		component.Description = req.Description
	}

	if err := s.repo.Update(ctx, component); err != nil {
		return nil, err
	}

	// Reload to get updated relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toSalaryComponentResponse(updated)
	return &response, nil
}

func (s *salarycomponentServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("salary component not found")
		}
		return err
	}

	return s.repo.Delete(ctx, id)
}

func toSalaryComponentResponse(component *model.SalaryComponent) model.SalaryComponentResponse {
	response := model.SalaryComponentResponse{
		ID:                component.ID,
		BujpID:            component.BujpID,
		Code:              component.Code,
		Name:              component.Name,
		Type:              component.Type,
		CalculationMethod: component.CalculationMethod,
		Amount:            component.Amount,
		IsActive:          component.IsActive,
		Description:       component.Description,
		CreatedAt:         component.CreatedAt,
		UpdatedAt:         component.UpdatedAt,
	}

	if component.Bujp != nil {
		bujpResponse := &model.BujpResponse{
			ID:      component.Bujp.ID,
			Code:    component.Bujp.Code,
			Name:    component.Bujp.Name,
			Address: component.Bujp.Address,
			Phone:   component.Bujp.Phone,
			Email:   component.Bujp.Email,
			Status:  component.Bujp.Status,
		}
		response.Bujp = bujpResponse
	}

	return response
}
