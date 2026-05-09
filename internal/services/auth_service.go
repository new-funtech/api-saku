package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/emailworker"
	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

const (
	resetOTPLength = 6
	resetOTPTTL    = 10 * time.Minute
)

var (
	ErrEmailNotRegistered = errors.New("email not registered")
	ErrEmailQueueFailed   = errors.New("email queue failed")
)

type AuthService interface {
	Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error)
	Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error)
	ForgotPassword(ctx context.Context, req model.ForgotPasswordRequest) error
	VerifyResetOTP(ctx context.Context, req model.VerifyResetOTPRequest) (*model.VerifyResetOTPResponse, error)
	ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error
}

type authServiceImpl struct {
	userRepo   repository.UserRepository
	broker     *rabbitmq.Client
	emailQueue string
}

func NewAuthService(repo repository.UserRepository, broker *rabbitmq.Client, emailQueue string) AuthService {
	if emailQueue == "" {
		emailQueue = "email.send"
	}
	return &authServiceImpl{
		userRepo:   repo,
		broker:     broker,
		emailQueue: emailQueue,
	}
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

// ===================== FORGOT / RESET PASSWORD =====================
//
// Flow 3 langkah:
//   1. ForgotPassword(email)              -> generate OTP 6 digit, simpan di DB
//                                             (hash bcrypt) + kirim via email.
//   2. VerifyResetOTP(email, otp)         -> cocokkan, balas reset_token (JWT
//                                             singkat, 10 menit, claim subject
//                                             "password-reset").
//   3. ResetPassword(reset_token, ...)    -> validasi token, set password baru,
//                                             invalidate OTP.

func (s *authServiceImpl) ForgotPassword(ctx context.Context, req model.ForgotPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return ErrEmailNotRegistered
	}

	otp, err := generateNumericOTP(resetOTPLength)
	if err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashed)
	expires := time.Now().Add(resetOTPTTL)
	user.ResetOTP = &hashedStr
	user.ResetOTPExpiresAt = &expires
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	job := emailworker.EmailJob{
		To:       []string{user.Email},
		Subject:  "Kode Reset Password SAKU",
		HTMLBody: buildResetOTPHTML(user.FullName, otp, int(resetOTPTTL.Minutes())),
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	if s.broker == nil {
		return ErrEmailQueueFailed
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.broker.Publish(pubCtx, s.emailQueue, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailQueueFailed, err)
	}
	return nil
}

func (s *authServiceImpl) VerifyResetOTP(ctx context.Context, req model.VerifyResetOTPRequest) (*model.VerifyResetOTPResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || user == nil {
		return nil, errors.New("invalid otp")
	}
	if user.ResetOTP == nil || user.ResetOTPExpiresAt == nil {
		return nil, errors.New("invalid otp")
	}
	if time.Now().After(*user.ResetOTPExpiresAt) {
		return nil, errors.New("otp expired")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.ResetOTP), []byte(req.OTP)); err != nil {
		return nil, errors.New("invalid otp")
	}

	token, err := jwtutil.GenerateResetToken(user.ID, user.Email, resetOTPTTL)
	if err != nil {
		return nil, err
	}
	return &model.VerifyResetOTPResponse{
		ResetToken: token,
		ExpiresIn:  int(resetOTPTTL.Seconds()),
	}, nil
}

func (s *authServiceImpl) ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error {
	claims, err := jwtutil.ValidateResetToken(req.ResetToken)
	if err != nil {
		return errors.New("invalid reset token")
	}
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashed)
	user.ResetOTP = nil
	user.ResetOTPExpiresAt = nil
	return s.userRepo.Update(ctx, user)
}

// generateNumericOTP -- 6-digit random kuat (crypto/rand) zero-padded.
func generateNumericOTP(length int) (string, error) {
	max := big.NewInt(1)
	for i := 0; i < length; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n), nil
}

func buildResetOTPHTML(fullName, otp string, ttlMin int) string {
	if fullName == "" {
		fullName = "Pengguna SAKU"
	}
	return fmt.Sprintf(`<!doctype html>
<html><body style="font-family:Arial,sans-serif;max-width:480px;margin:24px auto;color:#0f172a">
  <h2 style="color:#2563EB;margin:0 0 12px">Reset Password SAKU</h2>
  <p>Halo <b>%s</b>,</p>
  <p>Berikut kode OTP untuk reset password akun Anda:</p>
  <p style="font-size:32px;font-weight:800;letter-spacing:6px;background:#F1F5FF;padding:16px;border-radius:12px;text-align:center;color:#2563EB">%s</p>
  <p>Kode ini berlaku selama <b>%d menit</b>. Jangan bagikan kode ini kepada siapa pun.</p>
  <p style="color:#64748B;font-size:12px;margin-top:24px">Jika Anda tidak meminta reset password, abaikan email ini.</p>
</body></html>`, fullName, otp, ttlMin)
}
