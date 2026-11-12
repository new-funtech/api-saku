package utils

import (
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
)

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

// FormatDate formats time to YYYY-MM-DD string
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(constants.DateFormatYYYYMMDD)
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
