package model

import (
	"bytes"
	"strconv"
)

type NumberString string

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

	if _, err := strconv.ParseFloat(string(b), 64); err != nil {
		return err
	}
	*n = NumberString(b)
	return nil
}

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

func (n NumberString) String() string { return string(n) }

func (n NumberString) PtrString() *string {
	if n == "" {
		return nil
	}
	s := string(n)
	return &s
}
