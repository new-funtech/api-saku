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
	"golang.org/x/crypto/bcrypt"
)

const userProfileCacheTTL = 5 * time.Minute

func userProfileCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("user:profile:%s", id.String())
}

type UserService interface {
	GetAllUsers(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.UserResponse, *model.PaginationMeta, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.UserResponse, error)
	CreateUser(ctx context.Context, req model.CreateUserRequest) (*model.UserResponse, error)
	UpdateUser(ctx context.Context, id uuid.UUID, req model.UpdateUserRequest) (*model.UserResponse, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req model.UpdateProfileRequest) (*model.ProfileResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req model.ChangePasswordRequest) error
}

type userServiceImpl struct {
	repo           repository.UserRepository
	personnelRepo  repository.PersonnelRepository
	assignmentRepo repository.AssignmentRepository
	loanRepo       repository.LoanRepository
}

func NewUserService(repo repository.UserRepository, personnelRepo repository.PersonnelRepository, assignmentRepo repository.AssignmentRepository, loanRepo repository.LoanRepository) UserService {
	return &userServiceImpl{
		repo:           repo,
		personnelRepo:  personnelRepo,
		assignmentRepo: assignmentRepo,
		loanRepo:       loanRepo,
	}
}

func (s *userServiceImpl) GetAllUsers(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.UserResponse, *model.PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	users, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, nil, err
	}

	responses := make([]model.UserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, toUserResponse(u))
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	meta := &model.PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return responses, meta, nil
}

func (s *userServiceImpl) GetUserByID(ctx context.Context, id uuid.UUID) (*model.UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := toUserResponse(*user)
	return &resp, nil
}

func (s *userServiceImpl) CreateUser(ctx context.Context, req model.CreateUserRequest) (*model.UserResponse, error) {
	// Check for existing active user. We surface a more informative error
	// when the conflict is across BUJP tenants so that the company admin
	// understands the user was already onboarded elsewhere.
	existingUser, _ := s.repo.FindByEmail(ctx, req.Email)
	if existingUser != nil {
		if req.BujpID != nil && existingUser.BujpID != nil && *existingUser.BujpID != *req.BujpID {
			return nil, errors.New("email sudah terdaftar pada perusahaan lain. Hubungi super admin untuk memindahkan akun")
		}
		return nil, errors.New("email already exists")
	}

	// Create new user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	status := req.Status
	if status == "" {
		status = "active"
	}

	user := model.User{
		BujpID:   req.BujpID,
		FullName: req.FullName,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     role,
		Phone:    req.Phone,
		Photo:    req.Photo,
		Status:   status,
	}

	if err = s.repo.Create(ctx, &user); err != nil {
		return nil, err
	}

	resp := toUserResponse(user)
	return &resp, nil
}

func (s *userServiceImpl) UpdateUser(ctx context.Context, id uuid.UUID, req model.UpdateUserRequest) (*model.UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.BujpID != nil {
		user.BujpID = req.BujpID
	}
	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Email != nil {
		existingUser, _ := s.repo.FindByEmail(ctx, *req.Email)
		if existingUser != nil && existingUser.ID != id {
			return nil, errors.New("email already exists")
		}
		user.Email = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.Photo != nil {
		user.Photo = req.Photo
	}

	// Clear preloaded BUJP so GORM Save doesn't overwrite our bujp_id change
	// with the stale association's primary key.
	user.Bujp = nil

	if err = s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	utils.CacheDel(ctx, userProfileCacheKey(id))
	resp := toUserResponse(*user)
	return &resp, nil
}

func (s *userServiceImpl) DeleteUser(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Look up the personnel record (if any) so we can validate loan history
	// and cascade the deletion to personnel + assignments.
	personnel, _ := s.personnelRepo.FindByUserID(ctx, id)
	if personnel != nil && s.loanRepo != nil {
		if count, lerr := s.loanRepo.CountByPersonnelID(ctx, personnel.ID); lerr == nil && count > 0 {
			name := user.FullName
			if name == "" {
				name = user.Email
			}
			return fmt.Errorf("pengguna %s tidak dapat dihapus karena masih memiliki %d data pinjaman. Selesaikan atau batalkan pinjaman terlebih dahulu", name, count)
		}
	}

	if personnel != nil {
		if s.assignmentRepo != nil {
			_ = s.assignmentRepo.DeleteByPersonnelID(ctx, personnel.ID)
		}
		if derr := s.personnelRepo.Delete(ctx, personnel.ID); derr != nil {
			return fmt.Errorf("failed to delete personnel cascade: %w", derr)
		}
	}

	utils.CacheDel(ctx, userProfileCacheKey(id))
	return s.repo.Delete(ctx, id)
}

func toUserResponse(u model.User) model.UserResponse {
	return model.UserResponse{
		ID:        u.ID,
		BujpID:    u.BujpID,
		Email:     u.Email,
		Role:      u.Role,
		FullName:  u.FullName,
		Phone:     u.Phone,
		Photo:     u.Photo,
		Status:    u.Status,
		LastLogin: u.LastLogin,
		Bujp:      nil, // Will be populated by Preload if needed
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (s *userServiceImpl) GetProfile(ctx context.Context, userID uuid.UUID) (*model.ProfileResponse, error) {
	cacheKey := userProfileCacheKey(userID)
	var cached model.ProfileResponse
	if err := utils.CacheGet(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	user, err := s.repo.FindByIDWithRelations(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := &model.ProfileResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		FullName:  user.FullName,
		Phone:     user.Phone,
		Photo:     user.Photo,
		Status:    user.Status,
		BujpID:    user.BujpID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	if user.Bujp != nil {
		bujpName := user.Bujp.Name
		profile.BujpName = &bujpName
	}

	// Get personnel data if exists
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err == nil && personnel != nil {
		personnelResp := ToPersonnelResponse(personnel)
		profile.Personnel = &personnelResp

		// Get current assignment for guards
		if user.Role == "guard" {
			assignment, err := s.assignmentRepo.FindCurrentByPersonnelID(ctx, personnel.ID)
			if err == nil && assignment != nil {
				currentAssignment := &model.CurrentAssignmentDetail{
					ID:        assignment.ID,
					StartDate: &assignment.StartDate,
					EndDate:   assignment.EndDate,
					Status:    assignment.Status,
					Notes:     assignment.Notes,
				}

				if assignment.Shift != nil {
					currentAssignment.Shift = &model.ShiftDetail{
						ID:        assignment.Shift.ID,
						Name:      assignment.Shift.Name,
						StartTime: assignment.Shift.StartTime,
						EndTime:   assignment.Shift.EndTime,
					}
				}

				if assignment.Location != nil {
					location := assignment.Location
					address := location.Address
					radius := location.AttendanceRadius
					locationDetail := &model.LocationDetail{
						ID:               location.ID,
						Code:             location.Code,
						Name:             location.Name,
						Address:          &address,
						Latitude:         location.Latitude,
						Longitude:        location.Longitude,
						AttendanceRadius: &radius,
					}

					if location.Bujp != nil {
						bujpName := location.Bujp.Name
						locationDetail.BujpName = &bujpName
					}

					currentAssignment.Location = locationDetail
				}

				profile.CurrentAssignment = currentAssignment
			}
		}
	}

	_ = utils.CacheSet(ctx, cacheKey, profile, userProfileCacheTTL)
	return profile, nil
}

func (s *userServiceImpl) UpdateProfile(ctx context.Context, userID uuid.UUID, req model.UpdateProfileRequest) (*model.ProfileResponse, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Email != nil {
		existingUser, _ := s.repo.FindByEmail(ctx, *req.Email)
		if existingUser != nil && existingUser.ID != userID {
			return nil, errors.New("email already exists")
		}
		user.Email = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.Photo != nil {
		user.Photo = req.Photo
	}

	if err = s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	utils.CacheDel(ctx, userProfileCacheKey(userID))
	return s.GetProfile(ctx, userID)
}

func (s *userServiceImpl) ChangePassword(ctx context.Context, userID uuid.UUID, req model.ChangePasswordRequest) error {
	// Validate that new password matches confirmation
	if req.NewPassword != req.ConfirmPassword {
		return errors.New("new password and confirm password do not match")
	}

	// Get user from database
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	// Check that new password is different from current
	if req.CurrentPassword == req.NewPassword {
		return errors.New("new password must be different from current password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update password
	user.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	utils.CacheDel(ctx, userProfileCacheKey(userID))
	return nil
}
