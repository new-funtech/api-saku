package utils

import (
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/google/uuid"
)

// ParseUUID parses string to UUID with error handling
func ParseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID format: %v", err)
	}
	return id, nil
}

// ParseDate parses a date string. Accepts the standard YYYY-MM-DD format plus
// the most common ISO / form variants (matches Laravel's lenient "date" rule).
func ParseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("date string is empty")
	}
	if t, err := ParseFlexibleDate(dateStr); err == nil {
		return t, nil
	}
	// Fall back to the original strict format for the canonical error message.
	date, err := time.Parse(constants.DateFormatYYYYMMDD, dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %v", err)
	}
	return date, nil
}

// ParseDateTime parses datetime string in RFC3339 format
func ParseDateTime(dateTimeStr string) (time.Time, error) {
	if dateTimeStr == "" {
		return time.Time{}, fmt.Errorf("datetime string is empty")
	}
	dt, err := time.Parse(time.RFC3339, dateTimeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid datetime format, expected RFC3339: %v", err)
	}
	return dt, nil
}

// ParsePeriod parses period string in YYYY-MM format
func ParsePeriod(periodStr string) (time.Time, error) {
	if periodStr == "" {
		return time.Time{}, fmt.Errorf("period string is empty")
	}
	period, err := time.Parse(constants.DateFormatYYYYMM, periodStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid period format, expected YYYY-MM: %v", err)
	}
	return period, nil
}

// FormatDate formats time to YYYY-MM-DD string
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(constants.DateFormatYYYYMMDD)
}

// FormatPeriod formats time to YYYY-MM string
func FormatPeriod(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(constants.DateFormatYYYYMM)
}

// ValidatePagination validates and normalizes pagination parameters
func ValidatePagination(page, limit int) (int, int) {
	if page < constants.DefaultPage {
		page = constants.DefaultPage
	}
	if limit < 1 || limit > constants.MaxLimit {
		limit = constants.DefaultLimit
	}
	return page, limit
}

// ParsePagination reads `page` and `limit` (with `per_page` as a Laravel-style
// alias) from the Fiber query string and runs them through ValidatePagination.
// Centralising this avoids drift between handlers and silently accepts the
// per_page param sent by client-web/admin clients.
func ParsePagination(c paginationQuerySource) (int, int) {
	page := c.QueryInt("page", constants.DefaultPage)
	limit := c.QueryInt("limit", constants.DefaultLimit)
	if pp := c.QueryInt("per_page", 0); pp > 0 {
		limit = pp
	}
	return ValidatePagination(page, limit)
}

// paginationQuerySource is satisfied by *fiber.Ctx; declared as an interface
// so the helper stays in pkg/utils without importing fiber here.
type paginationQuerySource interface {
	QueryInt(key string, defaultValue ...int) int
}

// CalculatePaginationMeta calculates pagination metadata
func CalculatePaginationMeta(page, limit int, total int64) (int, bool, bool) {
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	hasNext := page < totalPages
	hasPrevious := page > 1
	return totalPages, hasNext, hasPrevious
}

// StringPtr returns pointer to string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns pointer to int
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns pointer to float64
func Float64Ptr(f float64) *float64 {
	return &f
}

// UUIDPtr returns pointer to UUID
func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// TimePtr returns pointer to time
func TimePtr(t time.Time) *time.Time {
	return &t
}

// BoolPtr returns pointer to bool
func BoolPtr(b bool) *bool {
	return &b
}

// DerefString safely dereferences string pointer
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DerefInt safely dereferences int pointer
func DerefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// DerefFloat64 safely dereferences float64 pointer
func DerefFloat64(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

// DerefBool safely dereferences bool pointer
func DerefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// CalculateWorkDuration calculates work duration in minutes between two times
func CalculateWorkDuration(checkIn, checkOut time.Time) int {
	if checkIn.IsZero() || checkOut.IsZero() {
		return 0
	}
	duration := checkOut.Sub(checkIn)
	return int(duration.Minutes())
}

// IsValidStatus validates status against allowed values
func IsValidStatus(status string, allowedStatuses []string) bool {
	for _, allowed := range allowedStatuses {
		if status == allowed {
			return true
		}
	}
	return false
}

// DefaultIfEmpty returns default value if string is empty
func DefaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
