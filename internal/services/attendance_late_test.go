package services

import (
	"testing"
	"time"
)

func TestIsLateCheckIn(t *testing.T) {
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local)
	start := time.Date(0, 1, 1, 8, 0, 0, 0, time.UTC) // shift starts at 08:00
	at := func(h, m int) time.Time { return time.Date(2026, 11, 12, h, m, 0, 0, time.Local) }
	cases := []struct {
		name string
		in   time.Time
		want bool
	}{
		{"early", at(7, 30), false},
		{"on the dot", at(8, 0), false},
		{"inside the grace period", at(8, 5), false},
		{"just past the grace period", at(8, 6), true},
		{"half past eight", at(8, 30), true},
	}
	for _, c := range cases {
		if got := isLateCheckIn(day, start, c.in, attendanceLateGraceMinutes); got != c.want {
			t.Errorf("%s: late = %v, want %v", c.name, got, c.want)
		}
	}
}
