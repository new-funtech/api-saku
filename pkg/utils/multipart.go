package utils

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// flexibleDateLayouts lists the date layouts accepted by ParseFlexibleDate, in priority order.
var flexibleDateLayouts = []string{
	"2006-01-02",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"02/01/2006",
	"01/02/2006",
	time.RFC3339,
}

// ParseFlexibleDate accepts the most common date string formats sent by clients
// (matches Laravel's lenient `date` validation rule). Returns the date truncated to day.
func ParseFlexibleDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrEmptyDate
	}
	for _, layout := range flexibleDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			y, m, d := t.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, ErrInvalidDate
}

// ErrEmptyDate / ErrInvalidDate are sentinel errors for ParseFlexibleDate.
var (
	ErrEmptyDate   = newDateErr("date is empty")
	ErrInvalidDate = newDateErr("invalid date format")
)

type dateErr struct{ msg string }

func (e *dateErr) Error() string { return e.msg }
func newDateErr(s string) error  { return &dateErr{msg: s} }

// FormString returns the trimmed form value or fallback when missing.
func FormString(c *fiber.Ctx, key string, fallback ...string) string {
	v := strings.TrimSpace(c.FormValue(key))
	if v == "" && len(fallback) > 0 {
		return fallback[0]
	}
	return v
}

// FormBool parses common boolean form values (true/1/yes/on).
func FormBool(c *fiber.Ctx, key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(c.FormValue(key)))
	switch v {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

// FormInt parses an integer form value. Returns fallback on parse error or empty.
func FormInt(c *fiber.Ctx, key string, fallback int) int {
	v := strings.TrimSpace(c.FormValue(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// FormFloat parses a float form value. Returns fallback on parse error or empty.
func FormFloat(c *fiber.Ctx, key string, fallback float64) float64 {
	v := strings.TrimSpace(c.FormValue(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// FormUUID parses a uuid form value. Returns uuid.Nil if missing or invalid.
func FormUUID(c *fiber.Ctx, key string) uuid.UUID {
	v := strings.TrimSpace(c.FormValue(key))
	if v == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// UploadFormFile uploads a single multipart file (if present) to S3/MinIO under the given folder.
// Returns nil pointer (and no error) when the file isn't included in the request.
func UploadFormFile(ctx context.Context, c *fiber.Ctx, field, folder string) (*string, error) {
	fh, err := c.FormFile(field)
	if err != nil || fh == nil {
		return nil, nil
	}
	key, err := UploadFileToS3(ctx, fh, folder)
	if err != nil {
		return nil, err
	}
	return &key, nil
}
