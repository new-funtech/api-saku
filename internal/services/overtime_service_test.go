package services

import (
	"testing"
	"time"
)

func TestValidateOvertime(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)
	cases := []struct {
		name                     string
		date, start, end, reason string
		wantErr                  bool
		wantHours                float64
	}{
		{"ok", "2026-11-12", "17:00", "20:30", "Menyelesaikan laporan", false, 3.5},
		{"end before start", "2026-11-12", "20:00", "18:00", "Menyelesaikan laporan", true, 0},
		{"too short", "2026-11-12", "17:00", "17:15", "Menyelesaikan laporan", true, 0},
		{"too long", "2026-11-12", "06:00", "19:00", "Menyelesaikan laporan", true, 0},
		{"too old", "2026-09-01", "17:00", "19:00", "Menyelesaikan laporan", true, 0},
		{"too far ahead", "2026-12-01", "17:00", "19:00", "Menyelesaikan laporan", true, 0},
		{"short reason", "2026-11-12", "17:00", "19:00", "ya", true, 0},
		{"bad clock", "2026-11-12", "5pm", "19:00", "Menyelesaikan laporan", true, 0},
	}
	for _, c := range cases {
		_, hours, err := validateOvertime(c.date, c.start, c.end, c.reason, now)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && hours != c.wantHours {
			t.Errorf("%s: hours = %v, want %v", c.name, hours, c.wantHours)
		}
	}
}

func TestOvertimePay(t *testing.T) {
	base := 17_300_000.0 // hourly wage = 100,000
	cases := []struct {
		name  string
		hours map[string]float64
		want  float64
	}{
		{"none", nil, 0},
		{"half an hour", map[string]float64{"d1": 0.5}, 75_000},
		{"one hour", map[string]float64{"d1": 1}, 150_000},
		{"three hours", map[string]float64{"d1": 3}, 150_000 + 2*200_000},
		{"two days add up", map[string]float64{"d1": 1, "d2": 2}, 150_000 + 150_000 + 200_000},
	}
	for _, c := range cases {
		got, _ := overtimePay(base, c.hours)
		if got != c.want {
			t.Errorf("%s: pay = %v, want %v", c.name, got, c.want)
		}
	}
	if pay, _ := overtimePay(0, map[string]float64{"d": 2}); pay != 0 {
		t.Errorf("no base salary must pay nothing, got %v", pay)
	}
}
