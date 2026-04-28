package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BujpService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.BujpResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.BujpResponse, error)
	GetByCode(ctx context.Context, code string) (*model.BujpResponse, error)
	Create(ctx context.Context, req *model.CreateBujpRequest) (*model.BujpResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateBujpRequest) (*model.BujpResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type bujpServiceImpl struct {
	repo repository.BujpRepository
}

func NewBujpService(repo repository.BujpRepository) BujpService {
	return &bujpServiceImpl{repo: repo}
}

func (s *bujpServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.BujpResponse, int64, error) {
	bujps, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get bujps: %w", err)
	}

	responses := make([]model.BujpResponse, len(bujps))
	for i, bujp := range bujps {
		responses[i] = toBujpResponse(&bujp)
	}

	return responses, total, nil
}

func (s *bujpServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.BujpResponse, error) {
	bujp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("bujp not found")
		}
		return nil, fmt.Errorf("failed to get bujp: %w", err)
	}

	response := toBujpResponse(bujp)
	return &response, nil
}

func (s *bujpServiceImpl) GetByCode(ctx context.Context, code string) (*model.BujpResponse, error) {
	bujp, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("bujp not found")
		}
		return nil, fmt.Errorf("failed to get bujp: %w", err)
	}

	response := toBujpResponse(bujp)
	return &response, nil
}

func (s *bujpServiceImpl) Create(ctx context.Context, req *model.CreateBujpRequest) (*model.BujpResponse, error) {
	// Check if code already exists
	existing, _ := s.repo.FindByCode(ctx, req.Code)
	if existing != nil {
		return nil, fmt.Errorf("bujp with code %s already exists", req.Code)
	}

	// Parse PKS validity start date if provided
	var pksValidityStart *time.Time
	if req.PksValidityStart != nil && *req.PksValidityStart != "" {
		parsed, err := time.Parse("2006-01-02", *req.PksValidityStart)
		if err != nil {
			return nil, errors.New("invalid pks_validity_start format, expected YYYY-MM-DD")
		}
		pksValidityStart = &parsed
	}

	// Parse PKS validity end date if provided
	var pksValidityEnd *time.Time
	if req.PksValidityEnd != nil && *req.PksValidityEnd != "" {
		parsed, err := time.Parse("2006-01-02", *req.PksValidityEnd)
		if err != nil {
			return nil, errors.New("invalid pks_validity_end format, expected YYYY-MM-DD")
		}
		pksValidityEnd = &parsed
	}

	// Validate date range if both dates are provided
	if pksValidityStart != nil && pksValidityEnd != nil && pksValidityEnd.Before(*pksValidityStart) {
		return nil, errors.New("pks_validity_end cannot be before pks_validity_start")
	}

	bujp := &model.Bujp{
		Code:             req.Code,
		Name:             req.Name,
		Address:          req.Address,
		Phone:            req.Phone,
		Email:            req.Email,
		PicName:          req.PicName,
		PicPhone:         req.PicPhone,
		PksNumber:        req.PksNumber,
		PksValidityStart: pksValidityStart,
		PksValidityEnd:   pksValidityEnd,
		PksDocument:      req.PksDocument,
		BjbAccountNumber: req.BjbAccountNumber,
		Status:           req.Status,
	}

	if bujp.Status == "" {
		bujp.Status = "active"
	}

	promoteTempUpload(ctx, bujp.PksDocument)

	if err := s.repo.Create(ctx, bujp); err != nil {
		return nil, fmt.Errorf("failed to create bujp: %w", err)
	}

	response := toBujpResponse(bujp)
	return &response, nil
}

func (s *bujpServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateBujpRequest) (*model.BujpResponse, error) {
	bujp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("bujp not found")
		}
		return nil, fmt.Errorf("failed to get bujp: %w", err)
	}

	// Check if code is being changed and if it already exists
	if req.Code != nil && *req.Code != bujp.Code {
		existing, _ := s.repo.FindByCode(ctx, *req.Code)
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("bujp with code %s already exists", *req.Code)
		}
		bujp.Code = *req.Code
	}

	if req.Name != nil {
		bujp.Name = *req.Name
	}
	if req.Address != nil {
		bujp.Address = req.Address
	}
	if req.Phone != nil {
		bujp.Phone = req.Phone
	}
	if req.Email != nil {
		bujp.Email = req.Email
	}
	if req.PicName != nil {
		bujp.PicName = req.PicName
	}
	if req.PicPhone != nil {
		bujp.PicPhone = req.PicPhone
	}
	if req.PksNumber != nil {
		bujp.PksNumber = req.PksNumber
	}
	// Parse PKS validity start date if provided
	if req.PksValidityStart != nil {
		if *req.PksValidityStart != "" {
			parsed, err := time.Parse("2006-01-02", *req.PksValidityStart)
			if err != nil {
				return nil, errors.New("invalid pks_validity_start format, expected YYYY-MM-DD")
			}
			bujp.PksValidityStart = &parsed
		} else {
			bujp.PksValidityStart = nil
		}
	}
	// Parse PKS validity end date if provided
	if req.PksValidityEnd != nil {
		if *req.PksValidityEnd != "" {
			parsed, err := time.Parse("2006-01-02", *req.PksValidityEnd)
			if err != nil {
				return nil, errors.New("invalid pks_validity_end format, expected YYYY-MM-DD")
			}
			bujp.PksValidityEnd = &parsed
		} else {
			bujp.PksValidityEnd = nil
		}
	}
	// Validate date range if both dates are provided
	if bujp.PksValidityStart != nil && bujp.PksValidityEnd != nil && bujp.PksValidityEnd.Before(*bujp.PksValidityStart) {
		return nil, errors.New("pks_validity_end cannot be before pks_validity_start")
	}
	if req.PksDocument != nil {
		bujp.PksDocument = req.PksDocument
		promoteTempUpload(ctx, bujp.PksDocument)
	}
	if req.BjbAccountNumber != nil {
		bujp.BjbAccountNumber = req.BjbAccountNumber
	}
	if req.Status != nil {
		bujp.Status = *req.Status
	}

	if err := s.repo.Update(ctx, bujp); err != nil {
		return nil, fmt.Errorf("failed to update bujp: %w", err)
	}

	response := toBujpResponse(bujp)
	return &response, nil
}

func (s *bujpServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("bujp not found")
		}
		return fmt.Errorf("failed to get bujp: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete bujp: %w", err)
	}

	return nil
}

func toBujpResponse(bujp *model.Bujp) model.BujpResponse {
	// Generate presigned URL for PKS document if it exists
	var pksDocumentUrl *string
	if bujp.PksDocument != nil && *bujp.PksDocument != "" {
		ctx := context.Background()
		url, err := utils.GeneratePresignedURL(ctx, *bujp.PksDocument, 60*time.Minute)
		if err != nil {
			log.Printf("bujp %s: failed to generate presigned URL for pks_document=%q: %v", bujp.ID, *bujp.PksDocument, err)
		} else if url != "" {
			pksDocumentUrl = &url
		}
	}

	return model.BujpResponse{
		ID:               bujp.ID,
		Code:             bujp.Code,
		Name:             bujp.Name,
		Address:          bujp.Address,
		Phone:            bujp.Phone,
		Email:            bujp.Email,
		PicName:          bujp.PicName,
		PicPhone:         bujp.PicPhone,
		PksNumber:        bujp.PksNumber,
		PksValidityStart: bujp.PksValidityStart,
		PksValidityEnd:   bujp.PksValidityEnd,
		PksDocument:      bujp.PksDocument,
		PksDocumentUrl:   pksDocumentUrl,
		BjbAccountNumber: bujp.BjbAccountNumber,
		Status:           bujp.Status,
		CreatedAt:        bujp.CreatedAt,
		UpdatedAt:        bujp.UpdatedAt,
	}
}

// promoteTempUpload moves a freshly-uploaded file from temp/<key> to <key>
// in S3 if it is still located in the staging area. It silently no-ops when
// the source object does not exist (already moved or non-temp paths).
func promoteTempUpload(ctx context.Context, finalKey *string) {
	if finalKey == nil || *finalKey == "" {
		return
	}
	tempKey := "temp/" + *finalKey
	if err := utils.MoveFileInS3(ctx, tempKey, *finalKey); err != nil {
		log.Printf("promoteTempUpload: %s -> %s: %v", tempKey, *finalKey, err)
	}
}
