package mailer

import (
	"strings"
	"testing"
)

func TestModeAcceptsTheUsualSpellings(t *testing.T) {
	cases := []struct {
		enc, port string
		want      Security
	}{
		{"ssl", "465", SecurityImplicitTLS},
		{"true", "465", SecurityImplicitTLS}, // the production value that used to hang
		{"tls", "587", SecurityStartTLS},
		{"starttls", "587", SecurityStartTLS},
		{"tls", "465", SecurityImplicitTLS},
		{"", "465", SecurityImplicitTLS},
		{"", "587", SecurityStartTLS},
		{"none", "25", SecurityNone},
		{"false", "25", SecurityNone},
	}
	for _, c := range cases {
		if got := (Config{Encryption: c.enc, Port: c.port}).Mode(); got != c.want {
			t.Errorf("Mode(%q, %s) = %s, want %s", c.enc, c.port, got, c.want)
		}
	}
}

func TestBuildMessageIsSafeForStrictServers(t *testing.T) {
	cfg := Config{FromAddress: "no-reply@example.com", FromName: "SAKU — HRMIS"}
	html := "<p>" + strings.Repeat("Gaji bersih Rp 5.000.000 ", 200) + "</p>" // one very long line
	out := string(buildMessage(cfg, Message{
		To:       []string{"a@example.com"},
		Subject:  "[SAKU] Slip Gaji November 2026 — Budi",
		HTMLBody: html,
	}))

	headers, body, _ := strings.Cut(out, "\r\n\r\n")
	for _, line := range strings.Split(headers, "\r\n") {
		for _, r := range line {
			if r > 127 {
				t.Fatalf("raw non-ASCII in header %q", line)
			}
		}
	}
	for _, want := range []string{"Subject: =?UTF-8?q?", "Date: ", "Message-ID: <", "Content-Transfer-Encoding: base64"} {
		if !strings.Contains(headers, want) {
			t.Errorf("headers missing %q:\n%s", want, headers)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(body), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("body line too long (%d)", len(line))
		}
	}
}
