package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CustomDate handles date parsing in YYYY-MM-DD format
type CustomDate struct {
	time.Time
}

// UnmarshalJSON parses date from YYYY-MM-DD or ISO datetime format
func (cd *CustomDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		cd.Time = time.Time{}
		return nil
	}

	// Try parsing as date only (YYYY-MM-DD)
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		cd.Time = t
		return nil
	}

	// Try parsing as ISO datetime (2008-04-10T00:00:00Z)
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		cd.Time = t
		return nil
	}

	// Try parsing ISO datetime without timezone
	t, err = time.Parse("2006-01-02T15:04:05", s)
	if err == nil {
		cd.Time = t
		return nil
	}

	return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD or ISO datetime)", s)
}

// MarshalJSON returns date in YYYY-MM-DD format
func (cd CustomDate) MarshalJSON() ([]byte, error) {
	if cd.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", cd.Time.Format("2006-01-02"))), nil
}

// Value implements driver.Valuer for database
func (cd CustomDate) Value() (driver.Value, error) {
	if cd.Time.IsZero() {
		return nil, nil
	}
	return cd.Time, nil
}

// TimeString handles PostgreSQL TIME columns properly
// Accepts "HH:MM" or "HH:MM:SS" format
type TimeString string

func (ts *TimeString) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	// Normalize format: if "HH:MM", convert to "HH:MM:SS"
	if len(str) == 5 && str[2] == ':' {
		str = str + ":00"
	}
	*ts = TimeString(str)
	return nil
}

func (ts TimeString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(ts))
}

// Value implements driver.Valuer for database storage
func (ts TimeString) Value() (driver.Value, error) {
	if ts == "" {
		return nil, nil
	}
	return string(ts), nil
}

// Scan implements sql.Scanner for database retrieval
func (ts *TimeString) Scan(value interface{}) error {
	if value == nil {
		*ts = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*ts = TimeString(v)
	case string:
		*ts = TimeString(v)
	case time.Time:
		*ts = TimeString(v.Format("15:04:05"))
	default:
		return fmt.Errorf("cannot scan %T into TimeString", value)
	}
	return nil
}

// Scan implements sql.Scanner for database
func (cd *CustomDate) Scan(value interface{}) error {
	if value == nil {
		cd.Time = time.Time{}
		return nil
	}

	if t, ok := value.(time.Time); ok {
		cd.Time = t
		return nil
	}

	return fmt.Errorf("cannot scan %T into CustomDate", value)
}

// FormatDate returns t formatted as YYYY-MM-DD, or empty string if zero.
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// FormatDatePtr returns a pointer to YYYY-MM-DD string, or nil if t is nil/zero.
func FormatDatePtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

// TimeOnly handles PostgreSQL TIME columns (time without date)
// Stores as time.Time but only uses HH:MM:SS portion
type TimeOnly struct {
	time.Time
}

// UnmarshalJSON parses time from "HH:MM" or "HH:MM:SS" format
func (to *TimeOnly) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		to.Time = time.Time{}
		return nil
	}

	// Normalize: if "HH:MM", add ":00"
	if len(s) == 5 && s[2] == ':' {
		s = s + ":00"
	}

	// Parse as time only (using a dummy date)
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return fmt.Errorf("invalid time format, expected HH:MM or HH:MM:SS: %v", err)
	}
	to.Time = t
	return nil
}

// MarshalJSON returns time in "HH:MM:SS" format
func (to TimeOnly) MarshalJSON() ([]byte, error) {
	if to.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", to.Time.Format("15:04:05"))), nil
}

// Value implements driver.Valuer for database storage
// Returns time.Time which PostgreSQL will store in TIME column correctly
func (to TimeOnly) Value() (driver.Value, error) {
	if to.Time.IsZero() {
		return nil, nil
	}
	// Return the time.Time - GORM will handle conversion to TIME type
	return to.Time, nil
}

// Scan implements sql.Scanner for database retrieval
func (to *TimeOnly) Scan(value interface{}) error {
	if value == nil {
		to.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		to.Time = v
	case []byte:
		// Parse time from string if database returns it as bytes
		t, err := time.Parse("15:04:05", string(v))
		if err != nil {
			return err
		}
		to.Time = t
	case string:
		// Parse time from string
		t, err := time.Parse("15:04:05", v)
		if err != nil {
			return err
		}
		to.Time = t
	default:
		return fmt.Errorf("cannot scan %T into TimeOnly", value)
	}
	return nil
}

// GormDataType tells GORM to use TIME column type
func (TimeOnly) GormDataType() string {
	return "time"
}

// CustomTime handles time parsing in HH:MM or HH:MM:SS format
// and converts to timestamp for database storage
type CustomTime struct {
	time.Time
}

// UnmarshalJSON parses time from HH:MM or HH:MM:SS format
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		ct.Time = time.Time{}
		return nil
	}

	// Normalize HH:MM to HH:MM:SS
	if len(s) == 5 && s[2] == ':' {
		s = s + ":00"
	}

	// Parse time and set it to today's date for timestamp storage
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return err
	}

	// Set to a reference date (2000-01-01) to make it a full timestamp
	// This allows PostgreSQL timestamptz columns to accept it
	ct.Time = time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	return nil
}

// MarshalJSON returns time in HH:MM:SS format
func (ct CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ct.Time.Format("15:04:05"))), nil
}

// Value implements driver.Valuer for database (returns full timestamp)
func (ct CustomTime) Value() (driver.Value, error) {
	if ct.Time.IsZero() {
		return nil, nil
	}
	return ct.Time, nil
}

// Scan implements sql.Scanner for database
func (ct *CustomTime) Scan(value interface{}) error {
	if value == nil {
		ct.Time = time.Time{}
		return nil
	}

	if t, ok := value.(time.Time); ok {
		ct.Time = t
		return nil
	}

	return fmt.Errorf("cannot scan %T into CustomTime", value)
}

// String returns time in HH:MM:SS format
func (ct CustomTime) String() string {
	if ct.Time.IsZero() {
		return ""
	}
	return ct.Time.Format("15:04:05")
}
