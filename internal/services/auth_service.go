package services

import (
	"context"
	"errors"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error)
	Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error)
}

type authServiceImpl struct {
	userRepo repository.UserRepository
}

func NewAuthService(repo repository.UserRepository) AuthService {
	return &authServiceImpl{userRepo: repo}
}

func (s *authServiceImpl) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: model.UserResponse{
			ID:       user.ID,
			BujpID:   user.BujpID,
			Email:    user.Email,
			Role:     user.Role,
			FullName: user.FullName,
			Phone:    user.Phone,
			Photo:    user.Photo,
			Status:   user.Status,
		},
	}, nil
}

func (s *authServiceImpl) Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error) {
	existing, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return nil, errors.New("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := model.User{
		FullName: req.FullName,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     "guard",
		Status:   "active",
	}

	if err := s.userRepo.Create(ctx, &user); err != nil {
		return nil, err
	}

	token, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: model.UserResponse{
			ID:       user.ID,
			BujpID:   user.BujpID,
			Email:    user.Email,
			Role:     user.Role,
			FullName: user.FullName,
			Phone:    user.Phone,
			Photo:    user.Photo,
			Status:   user.Status,
		},
	}, nil
}
