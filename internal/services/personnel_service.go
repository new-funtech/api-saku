package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PersonnelService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PersonnelResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.PersonnelResponse, error)
	GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.PersonnelResponse, error)
	Create(ctx context.Context, req *model.CreatePersonnelRequest) (*model.PersonnelResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdatePersonnelRequest) (*model.PersonnelResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type personnelServiceImpl struct {
	repo     repository.PersonnelRepository
	loanRepo repository.LoanRepository
	userRepo repository.UserRepository
}

func NewPersonnelService(repo repository.PersonnelRepository, loanRepo repository.LoanRepository, userRepo repository.UserRepository) PersonnelService {
	return &personnelServiceImpl{repo: repo, loanRepo: loanRepo, userRepo: userRepo}
}

func (s *personnelServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PersonnelResponse, int64, error) {
	personnels, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get personnels: %w", err)
	}

	responses := make([]model.PersonnelResponse, len(personnels))
	for i, personnel := range personnels {
		responses[i] = toPersonnelResponse(&personnel)
	}

	return responses, total, nil
}

func (s *personnelServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.PersonnelResponse, error) {
	personnel, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("personnel not found")
		}
		return nil, fmt.Errorf("failed to get personnel: %w", err)
	}

	response := toPersonnelResponse(personnel)
	return &response, nil
}

func (s *personnelServiceImpl) GetByBujpID(ctx context.Context, bujpID uuid.UUID) ([]model.PersonnelResponse, error) {
	personnels, err := s.repo.FindByBujpID(ctx, bujpID)
	if err != nil {
		return nil, fmt.Errorf("failed to get personnels by bujp: %w", err)
	}

	responses := make([]model.PersonnelResponse, len(personnels))
	for i, personnel := range personnels {
		responses[i] = toPersonnelResponse(&personnel)
	}

	return responses, nil
}

func (s *personnelServiceImpl) Create(ctx context.Context, req *model.CreatePersonnelRequest) (*model.PersonnelResponse, error) {
	// Check if ID number already exists
	existing, _ := s.repo.FindByIDNumber(ctx, req.IDNumber)
	if existing != nil {
		return nil, fmt.Errorf("personnel with ID number %s already exists", req.IDNumber)
	}

	if req.UserID != nil && s.userRepo != nil {
		if assigned, _ := s.repo.FindByUserID(ctx, *req.UserID); assigned != nil {
			return nil, fmt.Errorf("user is already assigned to another personnel")
		}
		if user, _ := s.userRepo.FindByID(ctx, *req.UserID); user != nil {
			email := user.Email
			req.Email = &email
		}
	}

	// Convert CustomDate to *time.Time
	var birthDate *time.Time
	if req.BirthDate != nil && !req.BirthDate.IsZero() {
		t := req.BirthDate.Time
		birthDate = &t
	}

	var contractEndDate *time.Time
	if req.ContractEndDate != nil && !req.ContractEndDate.IsZero() {
		t := req.ContractEndDate.Time
		contractEndDate = &t
	}

	personnel := &model.Personnel{
		BujpID:           req.BujpID,
		UserID:           req.UserID,
		IDNumber:         req.IDNumber,
		NIP:              req.NIP,
		FullName:         req.FullName,
		MotherMaidenName: req.MotherMaidenName,
		Photo:            req.Photo,
		BirthDate:        birthDate,
		Gender:           req.Gender,
		Address:          req.Address,
		Phone:            req.Phone,
		EmergencyPhone:   req.EmergencyPhone,
		Email:            req.Email,
		LicenseNumber:    req.LicenseNumber,
		JoinDate:         req.JoinDate.Time,
		ContractEndDate:  contractEndDate,
		Status:           req.Status,
		BankName:         req.BankName,
		AccountNumber:    req.AccountNumber,
		BaseSalary:       parseBaseSalary(req.BaseSalary),
	}

	if personnel.Status == "" {
		personnel.Status = "active"
	}

	if err := s.repo.Create(ctx, personnel); err != nil {
		return nil, fmt.Errorf("failed to create personnel: %w", err)
	}

	// Fetch the created personnel with relations
	created, _ := s.repo.FindByID(ctx, personnel.ID)
	response := toPersonnelResponse(created)
	return &response, nil
}

func (s *personnelServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdatePersonnelRequest) (*model.PersonnelResponse, error) {
	personnel, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("personnel not found")
		}
		return nil, fmt.Errorf("failed to get personnel: %w", err)
	}

	// Check if ID number is being changed and if it already exists
	if req.IDNumber != nil && *req.IDNumber != personnel.IDNumber {
		existing, _ := s.repo.FindByIDNumber(ctx, *req.IDNumber)
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("personnel with ID number %s already exists", *req.IDNumber)
		}
		personnel.IDNumber = *req.IDNumber
	}

	if req.NIP != nil {
		personnel.NIP = req.NIP
	}

	if req.BujpID != nil {
		personnel.BujpID = *req.BujpID
	}
	if req.UserID != nil {
		if personnel.UserID == nil || *personnel.UserID != *req.UserID {
			if assigned, _ := s.repo.FindByUserID(ctx, *req.UserID); assigned != nil && assigned.ID != id {
				return nil, fmt.Errorf("user is already assigned to another personnel")
			}
			if s.userRepo != nil {
				if user, _ := s.userRepo.FindByID(ctx, *req.UserID); user != nil {
					email := user.Email
					personnel.Email = &email
				}
			}
		}
		personnel.UserID = req.UserID
	}
	if req.FullName != nil {
		personnel.FullName = *req.FullName
	}
	if req.MotherMaidenName != nil {
		personnel.MotherMaidenName = req.MotherMaidenName
	}
	if req.Photo != nil {
		personnel.Photo = req.Photo
	}
	if req.BirthDate != nil {
		if !req.BirthDate.IsZero() {
			t := req.BirthDate.Time
			personnel.BirthDate = &t
		} else {
			personnel.BirthDate = nil
		}
	}
	if req.Gender != nil {
		personnel.Gender = req.Gender
	}
	if req.Address != nil {
		personnel.Address = req.Address
	}
	if req.Phone != nil {
		personnel.Phone = req.Phone
	}
	if req.EmergencyPhone != nil {
		personnel.EmergencyPhone = req.EmergencyPhone
	}
	if req.Email != nil {
		personnel.Email = req.Email
	}
	if req.LicenseNumber != nil {
		personnel.LicenseNumber = req.LicenseNumber
	}
	if req.JoinDate != nil {
		personnel.JoinDate = req.JoinDate.Time
	}
	if req.ContractEndDate != nil {
		if !req.ContractEndDate.IsZero() {
			t := req.ContractEndDate.Time
			personnel.ContractEndDate = &t
		} else {
			personnel.ContractEndDate = nil
		}
	}
	if req.Status != nil {
		personnel.Status = *req.Status
	}
	if req.BankName != nil {
		personnel.BankName = req.BankName
	}
	if req.AccountNumber != nil {
		personnel.AccountNumber = req.AccountNumber
	}
	if req.BaseSalary != nil {
		personnel.BaseSalary = parseBaseSalary(*req.BaseSalary)
	}

	if err := s.repo.Update(ctx, personnel); err != nil {
		return nil, fmt.Errorf("failed to update personnel: %w", err)
	}

	// Fetch the updated personnel with relations
	updated, _ := s.repo.FindByID(ctx, id)
	response := toPersonnelResponse(updated)
	return &response, nil
}

func (s *personnelServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	personnel, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("personnel not found")
		}
		return fmt.Errorf("failed to get personnel: %w", err)
	}

	// Block deletion when the personnel still has any loan history; this
	// surfaces a friendlier message than the raw FK constraint error.
	if s.loanRepo != nil {
		if count, lerr := s.loanRepo.CountByPersonnelID(ctx, id); lerr == nil && count > 0 {
			return fmt.Errorf("personel %s tidak dapat dihapus karena masih memiliki %d data pinjaman. Selesaikan atau batalkan pinjaman terlebih dahulu sebelum menghapus personel", personnel.FullName, count)
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete personnel: %w", err)
	}

	return nil
}

// parseBaseSalary converts string to float64 for base_salary field
// Handles both string ("12000000") and number formats from JSON
func parseBaseSalary(salaryStr string) float64 {
	if salaryStr == "" {
		return 0
	}
	salary, err := strconv.ParseFloat(salaryStr, 64)
	if err != nil {
		return 0
	}
	return salary
}

func toPersonnelResponse(personnel *model.Personnel) model.PersonnelResponse {
	return ToPersonnelResponse(personnel)
}

func ToPersonnelResponse(personnel *model.Personnel) model.PersonnelResponse {
	response := model.PersonnelResponse{
		ID:               personnel.ID,
		BujpID:           personnel.BujpID,
		UserID:           personnel.UserID,
		IDNumber:         personnel.IDNumber,
		NIP:              personnel.NIP,
		FullName:         personnel.FullName,
		MotherMaidenName: personnel.MotherMaidenName,
		Photo:            personnel.Photo,
		BirthDate:        model.FormatDatePtr(personnel.BirthDate),
		Gender:           personnel.Gender,
		Address:          personnel.Address,
		Phone:            personnel.Phone,
		EmergencyPhone:   personnel.EmergencyPhone,
		Email:            personnel.Email,
		LicenseNumber:    personnel.LicenseNumber,
		JoinDate:         model.FormatDate(personnel.JoinDate),
		ContractEndDate:  model.FormatDatePtr(personnel.ContractEndDate),
		Status:           personnel.Status,
		BankName:         personnel.BankName,
		AccountNumber:    personnel.AccountNumber,
		BaseSalary:       personnel.BaseSalary,
		CreatedAt:        personnel.CreatedAt,
		UpdatedAt:        personnel.UpdatedAt,
	}

	if personnel.Bujp != nil {
		bujpResponse := toBujpResponse(personnel.Bujp)
		response.Bujp = &bujpResponse
	}

	if personnel.User != nil {
		userResponse := model.UserResponse{
			ID:        personnel.User.ID,
			BujpID:    personnel.User.BujpID,
			Email:     personnel.User.Email,
			Role:      personnel.User.Role,
			FullName:  personnel.User.FullName,
			Phone:     personnel.User.Phone,
			Photo:     personnel.User.Photo,
			Status:    personnel.User.Status,
			LastLogin: personnel.User.LastLogin,
			CreatedAt: personnel.User.CreatedAt,
			UpdatedAt: personnel.User.UpdatedAt,
		}
		response.User = &userResponse
	}

	return response
}
