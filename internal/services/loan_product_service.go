package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoanProductService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanProductResponse, int64, error)
	GetActive(ctx context.Context, bujpID *uuid.UUID) ([]model.LoanProductResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.LoanProductResponse, error)
	Create(ctx context.Context, req *model.CreateLoanProductRequest, createdBy *uuid.UUID) (*model.LoanProductResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateLoanProductRequest) (*model.LoanProductResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type loanProductServiceImpl struct {
	repo repository.LoanProductRepository
}

func NewLoanProductService(repo repository.LoanProductRepository) LoanProductService {
	return &loanProductServiceImpl{repo: repo}
}

func (s *loanProductServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.LoanProductResponse, int64, error) {
	items, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get loan products: %w", err)
	}
	out := make([]model.LoanProductResponse, len(items))
	for i := range items {
		out[i] = ToLoanProductResponse(&items[i])
	}
	return out, total, nil
}

func (s *loanProductServiceImpl) GetActive(ctx context.Context, bujpID *uuid.UUID) ([]model.LoanProductResponse, error) {
	items, err := s.repo.FindActive(ctx, bujpID)
	if err != nil {
		return nil, err
	}
	out := make([]model.LoanProductResponse, len(items))
	for i := range items {
		out[i] = ToLoanProductResponse(&items[i])
	}
	return out, nil
}

func (s *loanProductServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.LoanProductResponse, error) {
	lp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("loan product not found")
		}
		return nil, err
	}
	r := ToLoanProductResponse(lp)
	return &r, nil
}

func (s *loanProductServiceImpl) Create(ctx context.Context, req *model.CreateLoanProductRequest, createdBy *uuid.UUID) (*model.LoanProductResponse, error) {
	code, err := s.generateCode(ctx)
	if err != nil {
		return nil, err
	}
	lp := &model.LoanProduct{
		BujpID:            req.BujpID,
		Code:              code,
		Name:              req.Name,
		Description:       req.Description,
		MinAmount:         req.MinAmount,
		MaxAmount:         req.MaxAmount,
		InterestRate:      req.InterestRate,
		MaxTenor:          req.MaxTenor,
		RequirePks:        derefBoolDefault(req.RequirePks, true),
		RequireCollateral: derefBoolDefault(req.RequireCollateral, false),
		IsActive:          derefBoolDefault(req.IsActive, true),
		CreatedBy:         createdBy,
	}
	if err := s.repo.Create(ctx, lp); err != nil {
		return nil, err
	}
	r := ToLoanProductResponse(lp)
	return &r, nil
}

func (s *loanProductServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateLoanProductRequest) (*model.LoanProductResponse, error) {
	lp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("loan product not found")
		}
		return nil, err
	}
	if req.BujpID != nil {
		lp.BujpID = req.BujpID
	}
	if req.Name != nil {
		lp.Name = *req.Name
	}
	if req.Description != nil {
		lp.Description = req.Description
	}
	if req.MinAmount != nil {
		lp.MinAmount = *req.MinAmount
	}
	if req.MaxAmount != nil {
		lp.MaxAmount = *req.MaxAmount
	}
	if req.InterestRate != nil {
		lp.InterestRate = *req.InterestRate
	}
	if req.MaxTenor != nil {
		lp.MaxTenor = *req.MaxTenor
	}
	if req.RequirePks != nil {
		lp.RequirePks = *req.RequirePks
	}
	if req.RequireCollateral != nil {
		lp.RequireCollateral = *req.RequireCollateral
	}
	if req.IsActive != nil {
		lp.IsActive = *req.IsActive
	}
	if err := s.repo.Update(ctx, lp); err != nil {
		return nil, err
	}
	r := ToLoanProductResponse(lp)
	return &r, nil
}

func (s *loanProductServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("loan product not found")
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// generateCode produces LPN-YYMM-XXXX.
func (s *loanProductServiceImpl) generateCode(ctx context.Context) (string, error) {
	count, err := s.repo.CountThisMonth(ctx)
	if err != nil {
		return "", err
	}
	now := time.Now()
	return fmt.Sprintf("LPN-%02d%02d-%04d", now.Year()%100, now.Month(), count+1), nil
}

// === Mappers ===

func ToLoanProductResponse(lp *model.LoanProduct) model.LoanProductResponse {
	r := model.LoanProductResponse{
		ID:                lp.ID,
		BujpID:            lp.BujpID,
		Code:              lp.Code,
		Name:              lp.Name,
		Description:       lp.Description,
		MinAmount:         lp.MinAmount,
		MaxAmount:         lp.MaxAmount,
		InterestRate:      lp.InterestRate,
		MaxTenor:          lp.MaxTenor,
		RequirePks:        lp.RequirePks,
		RequireCollateral: lp.RequireCollateral,
		IsActive:          lp.IsActive,
		CreatedBy:         lp.CreatedBy,
		CreatedAt:         lp.CreatedAt,
		UpdatedAt:         lp.UpdatedAt,
	}
	if lp.Bujp != nil {
		b := toBujpResponse(lp.Bujp)
		r.Bujp = &b
	}
	if lp.Creator != nil {
		c := toUserResponse(*lp.Creator)
		r.Creator = &c
	}
	return r
}

func derefBoolDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
