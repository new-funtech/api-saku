package model

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

type RegisterRequest struct {
	FullName string `json:"full_name" validate:"required" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required,min=6" example:"password123"`
}

type AuthResponse struct {
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserResponse `json:"user"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email" example:"john@example.com"`
}

type VerifyResetOTPRequest struct {
	Email string `json:"email" validate:"required,email" example:"john@example.com"`
	OTP   string `json:"otp" validate:"required,len=6,numeric" example:"123456"`
}

type VerifyResetOTPResponse struct {
	ResetToken string `json:"reset_token"`
	ExpiresIn  int    `json:"expires_in" example:"600"`
}

type ResetPasswordRequest struct {
	ResetToken      string `json:"reset_token" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
	ConfirmPassword string `json:"new_password_confirmation" validate:"required,eqfield=NewPassword"`
}
