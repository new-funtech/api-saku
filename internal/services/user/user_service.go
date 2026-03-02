package user

import (
	"errors"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	userRepo "github.com/ganiramadhan/ganipedia/backend/internal/repository/user"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	GetAllUsers(page, limit int) ([]model.UserResponse, *model.PaginationMeta, error)
	GetUserByID(id uuid.UUID) (*model.UserResponse, error)
	CreateUser(req model.CreateUserRequest) (*model.UserResponse, error)
	UpdateUser(id uuid.UUID, req model.UpdateUserRequest) (*model.UserResponse, error)
	DeleteUser(id uuid.UUID) error
}

type service struct {
	repo userRepo.Repository
}

func NewService(repo userRepo.Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetAllUsers(page, limit int) ([]model.UserResponse, *model.PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	users, total, err := s.repo.FindAll(page, limit)
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

func (s *service) GetUserByID(id uuid.UUID) (*model.UserResponse, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	resp := toUserResponse(*user)
	return &resp, nil
}

func (s *service) CreateUser(req model.CreateUserRequest) (*model.UserResponse, error) {
	existingUser, _ := s.repo.FindByEmail(req.Email)
	if existingUser != nil {
		return nil, errors.New("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	user := model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     role,
	}

	if err = s.repo.Create(&user); err != nil {
		return nil, err
	}

	resp := toUserResponse(user)
	return &resp, nil
}

func (s *service) UpdateUser(id uuid.UUID, req model.UpdateUserRequest) (*model.UserResponse, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		existingUser, _ := s.repo.FindByEmail(req.Email)
		if existingUser != nil && existingUser.ID != id {
			return nil, errors.New("email already exists")
		}
		user.Email = req.Email
	}
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}
	if req.Role != "" {
		user.Role = req.Role
	}

	if err = s.repo.Update(user); err != nil {
		return nil, err
	}

	resp := toUserResponse(*user)
	return &resp, nil
}

func (s *service) DeleteUser(id uuid.UUID) error {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func toUserResponse(u model.User) model.UserResponse {
	return model.UserResponse{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
		Role:  u.Role,
	}
}
