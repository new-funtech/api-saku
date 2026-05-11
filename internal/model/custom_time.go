package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CustomDate struct {
	time.Time
}

func (cd *CustomDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		cd.Time = time.Time{}
		return nil
	}

	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		cd.Time = t
		return nil
	}

	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		cd.Time = t
		return nil
	}

	t, err = time.Parse("2006-01-02T15:04:05", s)
	if err == nil {
		cd.Time = t
		return nil
	}

	return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD or ISO datetime)", s)
}

func (cd CustomDate) MarshalJSON() ([]byte, error) {
	if cd.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", cd.Time.Format("2006-01-02"))), nil
}

func (cd CustomDate) Value() (driver.Value, error) {
	if cd.Time.IsZero() {
		return nil, nil
	}
	return cd.Time, nil
}

type TimeString string

func (ts *TimeString) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}

	if len(str) == 5 && str[2] == ':' {
		str = str + ":00"
	}

	*ts = TimeString(str)
	return nil
}

func (ts TimeString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(ts))
}

func (ts TimeString) Value() (driver.Value, error) {
	if ts == "" {
		return nil, nil
	}
	return string(ts), nil
}

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

func FormatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func FormatDatePtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}

	s := t.Format("2006-01-02")
	return &s
}

type TimeOnly struct {
	time.Time
}

func (to *TimeOnly) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		to.Time = time.Time{}
		return nil
	}

	if len(s) == 5 && s[2] == ':' {
		s = s + ":00"
	}

	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return fmt.Errorf("invalid time format, expected HH:MM or HH:MM:SS: %v", err)
	}

	to.Time = t
	return nil
}

func (to TimeOnly) MarshalJSON() ([]byte, error) {
	if to.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", to.Time.Format("15:04:05"))), nil
}

func (to TimeOnly) Value() (driver.Value, error) {
	if to.Time.IsZero() {
		return nil, nil
	}
	return to.Time, nil
}

func (to *TimeOnly) Scan(value interface{}) error {
	if value == nil {
		to.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		to.Time = v
	case []byte:
		t, err := time.Parse("15:04:05", string(v))
		if err != nil {
			return err
		}
		to.Time = t
	case string:
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

func (TimeOnly) GormDataType() string {
	return "time"
}

type CustomTime struct {
	time.Time
}

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		ct.Time = time.Time{}
		return nil
	}

	if len(s) == 5 && s[2] == ':' {
		s = s + ":00"
	}

	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return err
	}

	ct.Time = time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	return nil
}

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ct.Time.Format("15:04:05"))), nil
}

func (ct CustomTime) Value() (driver.Value, error) {
	if ct.Time.IsZero() {
		return nil, nil
	}
	return ct.Time, nil
}

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

func (ct CustomTime) String() string {
	if ct.Time.IsZero() {
		return ""
	}
	return ct.Time.Format("15:04:05")
}
