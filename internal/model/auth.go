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
