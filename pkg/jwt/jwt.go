package jwt

import (
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwtlib.RegisteredClaims
}

func GenerateToken(userID uuid.UUID, email, role string) (string, error) {
	secret := config.GetEnvRequired("JWT_SECRET")

	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	secret := config.GetEnvRequired("JWT_SECRET")

	token, err := jwtlib.ParseWithClaims(tokenString, &Claims{}, func(token *jwtlib.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ===================== RESET PASSWORD TOKEN =====================
//
// Token singkat untuk flow reset password. Subject "password-reset" supaya
// tidak bisa dipakai untuk autentikasi route lain. TTL diatur caller (mis.
// 10 menit). Role di claim dikosongkan supaya kalau toh dipakai sebagai
// auth header tetap tidak punya akses.
const resetTokenSubject = "password-reset"

func GenerateResetToken(userID uuid.UUID, email string, ttl time.Duration) (string, error) {
	secret := config.GetEnvRequired("JWT_SECRET")
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   "",
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   resetTokenSubject,
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateResetToken(tokenString string) (*Claims, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Subject != resetTokenSubject {
		return nil, errors.New("not a reset token")
	}
	return claims, nil
}
