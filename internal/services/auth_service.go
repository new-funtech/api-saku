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
	otpDigits := ""
	for i, ch := range otp {
		if i > 0 {
			otpDigits += `<span style="display:inline-block;width:10px"></span>`
		}
		otpDigits += fmt.Sprintf(`<span style="display:inline-block;min-width:48px;padding:18px 0;background:#ffffff;border:1.5px solid #bfdbfe;border-radius:12px;font-size:30px;font-weight:700;color:#1d4ed8;font-family:'SF Mono','Cascadia Code','Courier New',monospace;letter-spacing:0;box-shadow:0 1px 2px rgba(37,99,235,0.06)">%c</span>`, ch)
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>Reset Password SAKU</title>
<style>@media only screen and (max-width:620px){.email-wrapper{width:100%%!important}.body-pad{padding:24px 20px!important}.header-pad{padding:26px 22px!important}.otp-box{min-width:42px!important;font-size:24px!important;padding:14px 0!important}}</style>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'SF Pro Display','SF Pro Text','Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale;color:#0f172a">
<div style="display:none;max-height:0;overflow:hidden;font-size:1px;line-height:1px;color:#f1f5f9">Kode OTP reset password Anda berlaku %d menit. Jangan bagikan kepada siapa pun.</div>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f1f5f9">
<tr><td align="center" style="padding:32px 16px">
<table role="presentation" class="email-wrapper" width="600" cellpadding="0" cellspacing="0" style="width:600px;max-width:100%%;background-color:#ffffff;border-radius:18px;overflow:hidden;box-shadow:0 1px 2px rgba(15,23,42,0.04),0 8px 24px rgba(15,23,42,0.06);border:1px solid #e2e8f0">

<!-- Brand bar -->
<tr><td style="background:linear-gradient(135deg,#3b82f6 0%%,#2563eb 100%%);padding:6px 0"></td></tr>

<!-- Header -->
<tr><td class="header-pad" style="padding:30px 36px 26px;border-bottom:1px solid #f1f5f9">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr>
<td valign="top">
<p style="margin:0 0 8px;font-size:11px;font-weight:600;color:#94a3b8;letter-spacing:1.4px;text-transform:uppercase">SAKU &middot; Aplikasi Kepegawaian</p>
<h1 style="margin:0 0 6px;font-size:24px;font-weight:700;color:#0f172a;line-height:1.2;letter-spacing:-0.4px">Reset Password</h1>
<p style="margin:0;font-size:13.5px;color:#64748b;line-height:1.5">Permintaan pengaturan ulang kata sandi akun Anda.</p>
</td>
<td align="right" valign="top" style="padding-left:12px"><span style="display:inline-block;background-color:#eff6ff;color:#1d4ed8;font-size:11px;font-weight:600;padding:6px 14px;border-radius:999px;border:1px solid #bfdbfe;white-space:nowrap;letter-spacing:0.3px">Verifikasi</span></td>
</tr></table>
</td></tr>

<!-- Body -->
<tr><td class="body-pad" style="padding:28px 36px 8px">

<p style="margin:0 0 14px;font-size:15px;color:#0f172a;line-height:1.55">Halo, <strong>%s</strong> &#128075;</p>
<p style="margin:0 0 22px;font-size:14.5px;color:#475569;line-height:1.65">Kami menerima permintaan untuk mengatur ulang kata sandi akun SAKU Anda. Gunakan kode verifikasi sekali pakai (OTP) di bawah ini untuk melanjutkan proses.</p>

<!-- OTP Hero -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:linear-gradient(135deg,#eff6ff 0%%,#dbeafe 100%%);border:1px solid #bfdbfe;border-radius:14px;margin:0 0 24px"><tr><td style="padding:26px 20px;text-align:center">
<p style="margin:0 0 14px;font-size:11px;font-weight:600;color:#1e40af;text-transform:uppercase;letter-spacing:1px">Kode Verifikasi Anda</p>
<div style="font-size:0;line-height:0">%s</div>
<p style="margin:18px 0 0;font-size:13px;color:#1e40af">Berlaku selama <strong style="color:#1d4ed8">%d menit</strong> sejak email ini dikirim</p>
</td></tr></table>

<!-- Steps -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:12px;overflow:hidden;margin:0 0 22px">
<tr><td style="padding:14px 18px 6px;background-color:#fafbfc">
<p style="margin:0;font-size:11px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:0.6px">Langkah Selanjutnya</p>
</td></tr>
<tr><td style="padding:8px 18px 16px;background-color:#fafbfc">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
<tr><td style="padding:6px 0;width:28px;vertical-align:top"><span style="display:inline-block;width:22px;height:22px;background-color:#2563eb;color:#ffffff;border-radius:50%%;font-size:11px;font-weight:700;line-height:22px;text-align:center">1</span></td><td style="padding:6px 0;font-size:13.5px;color:#334155;line-height:1.55">Buka aplikasi atau halaman <strong>Reset Password</strong> SAKU.</td></tr>
<tr><td style="padding:6px 0;vertical-align:top"><span style="display:inline-block;width:22px;height:22px;background-color:#2563eb;color:#ffffff;border-radius:50%%;font-size:11px;font-weight:700;line-height:22px;text-align:center">2</span></td><td style="padding:6px 0;font-size:13.5px;color:#334155;line-height:1.55">Masukkan kode <strong>6 digit</strong> di atas pada kolom OTP.</td></tr>
<tr><td style="padding:6px 0;vertical-align:top"><span style="display:inline-block;width:22px;height:22px;background-color:#2563eb;color:#ffffff;border-radius:50%%;font-size:11px;font-weight:700;line-height:22px;text-align:center">3</span></td><td style="padding:6px 0;font-size:13.5px;color:#334155;line-height:1.55">Buat kata sandi baru yang kuat dan mudah Anda ingat.</td></tr>
</table>
</td></tr>
</table>

<!-- Security notice -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#fffbeb;border:1px solid #fde68a;border-radius:12px;margin:0 0 18px"><tr><td style="padding:14px 18px">
<p style="margin:0 0 4px;font-size:13px;font-weight:700;color:#854d0e">&#128274; Jaga kerahasiaan kode Anda</p>
<p style="margin:0;font-size:13px;color:#854d0e;line-height:1.6">Tim SAKU <strong>tidak akan pernah</strong> meminta kode OTP Anda melalui telepon, WhatsApp, atau saluran apa pun. Jangan bagikan kepada siapa pun, termasuk admin perusahaan.</p>
</td></tr></table>

<!-- Not you? -->
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#fef2f2;border:1px solid #fecaca;border-radius:12px;margin:0 0 8px"><tr><td style="padding:14px 18px">
<p style="margin:0 0 4px;font-size:13px;font-weight:700;color:#991b1b">Bukan Anda yang meminta?</p>
<p style="margin:0;font-size:13px;color:#991b1b;line-height:1.6">Abaikan email ini &mdash; kata sandi Anda <strong>tidak akan berubah</strong> selama Anda tidak menggunakan kode di atas. Untuk keamanan ekstra, segera hubungi admin perusahaan Anda.</p>
</td></tr></table>

</td></tr>

<!-- Footer -->
<tr><td style="padding:22px 36px 26px;border-top:1px solid #f1f5f9;background-color:#fafbfc;text-align:center">
<p style="margin:0 0 6px;font-size:12px;color:#94a3b8;line-height:1.6">Email ini dikirim otomatis &mdash; harap tidak membalas pesan ini.</p>
<p style="margin:0 0 4px;font-size:11.5px;color:#94a3b8">Untuk bantuan, hubungi admin perusahaan Anda.</p>
<p style="margin:14px 0 0;font-size:11px;color:#cbd5e1;letter-spacing:0.3px">&copy; %d SAKU &bull; Aplikasi Kepegawaian</p>
</td></tr>

</table>
</td></tr></table>
</body></html>`, ttlMin, fullName, otpDigits, ttlMin, time.Now().Year())
}
