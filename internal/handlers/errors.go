package handlers

import (
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

// forbiddenResponse returns a uniform 403 to deny cross-tenant access.
// It avoids leaking whether a record exists by using the same message.
func forbiddenResponse(c *fiber.Ctx) error {
	return c.Status(http.StatusForbidden).JSON(model.APIResponse{
		Status:  "error",
		Code:    http.StatusForbidden,
		Message: "Anda tidak memiliki akses ke data ini",
	})
}

func statusForServiceError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "already exists"),
		strings.Contains(msg, "duplicate"),
		strings.Contains(msg, "overlapping"),
		strings.Contains(msg, "bertabrakan"),
		strings.Contains(msg, "sudah ada"),
		strings.Contains(msg, "sudah terdaftar"),
		strings.Contains(msg, "tidak dapat dihapus"),
		strings.Contains(msg, "tidak bisa dihapus"),
		strings.Contains(msg, "masih memiliki"),
		strings.Contains(msg, "memiliki pinjaman"),
		strings.Contains(msg, "memiliki riwayat"):
		return http.StatusConflict
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "invalid"),
		strings.Contains(msg, "required"),
		strings.Contains(msg, "must be"),
		strings.Contains(msg, "cannot be"),
		strings.Contains(msg, "belum waktunya"),
		strings.Contains(msg, "belum boleh"):
		return http.StatusBadRequest
	case strings.Contains(msg, "forbidden"),
		strings.Contains(msg, "not allowed"),
		strings.Contains(msg, "permission"):
		return http.StatusForbidden
	case strings.Contains(msg, "unauthorized"):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
