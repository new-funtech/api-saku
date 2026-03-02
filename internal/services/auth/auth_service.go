package auth

import (
	"errors"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	userRepo "github.com/ganiramadhan/ganipedia/backend/internal/repository/user"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Login(req model.LoginRequest) (*model.AuthResponse, error)
	Register(req model.RegisterRequest) (*model.AuthResponse, error)
}

type service struct {
	userRepo userRepo.Repository
}

func NewService(userRepo userRepo.Repository) Service {
	return &service{userRepo: userRepo}
}

func (s *service) Login(req model.LoginRequest) (*model.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
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
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}

func (s *service) Register(req model.RegisterRequest) (*model.AuthResponse, error) {
	existing, _ := s.userRepo.FindByEmail(req.Email)
	if existing != nil {
		return nil, errors.New("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     "user",
	}

	if err := s.userRepo.Create(&user); err != nil {
		return nil, err
	}

	token, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: model.UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}
