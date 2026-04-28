package model

import (
	"bytes"
	"strconv"
)

// NumberString is a JSON-friendly string that also accepts JSON numbers.
// Useful for fields like latitude/longitude which clients may send either as
// a JSON string (e.g. "-6.123") or as a raw number (e.g. -6.123).
type NumberString string

// UnmarshalJSON accepts either a JSON string, a JSON number, or null.
func (n *NumberString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*n = ""
		return nil
	}
	if b[0] == '"' && b[len(b)-1] == '"' {
		*n = NumberString(b[1 : len(b)-1])
		return nil
	}
	// Treat as a number — keep the raw textual representation so we don't lose
	// precision (the DB column is decimal).
	if _, err := strconv.ParseFloat(string(b), 64); err != nil {
		return err
	}
	*n = NumberString(b)
	return nil
}

// MarshalJSON renders the value as a JSON string. Empty values become null.
func (n NumberString) MarshalJSON() ([]byte, error) {
	if n == "" {
		return []byte("null"), nil
	}
	out := make([]byte, 0, len(n)+2)
	out = append(out, '"')
	out = append(out, []byte(n)...)
	out = append(out, '"')
	return out, nil
}

// String returns the underlying value as a regular Go string.
func (n NumberString) String() string { return string(n) }

// PtrString returns a *string pointing to the value, or nil for empty.
func (n NumberString) PtrString() *string {
	if n == "" {
		return nil
	}
	s := string(n)
	return &s
}
