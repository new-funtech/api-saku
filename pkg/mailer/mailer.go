// Package mailer provides a small SMTP helper used by the HRMIS API to send
// transactional emails (e.g. payroll slip notifications). It intentionally
// relies only on the Go standard library so it does not introduce new
// dependencies, and it is written to be a no-op when SMTP is not configured —
// callers can fire-and-forget without blowing up in dev/test environments.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
)

// Config captures the SMTP credentials and sender identity loaded from env.
type Config struct {
	Host        string
	Port        string
	Username    string
	Password    string
	Encryption  string // "ssl", "tls", or empty for plain
	FromAddress string
	FromName    string
}

// LoadConfig reads MAIL_* env variables into a Config.
func LoadConfig() Config {
	return Config{
		Host:        config.GetEnv("MAIL_HOST", ""),
		Port:        config.GetEnv("MAIL_PORT", "587"),
		Username:    config.GetEnv("MAIL_USERNAME", ""),
		Password:    config.GetEnv("MAIL_PASSWORD", ""),
		Encryption:  strings.ToLower(config.GetEnv("MAIL_ENCRYPTION", "tls")),
		FromAddress: config.GetEnv("MAIL_FROM_ADDRESS", ""),
		FromName:    config.GetEnv("MAIL_FROM_NAME", "HRMIS"),
	}
}

// IsEnabled returns true when the minimum SMTP fields are present so callers
// can short-circuit without attempting a network connection.
func (c Config) IsEnabled() bool {
	return c.Host != "" && c.FromAddress != ""
}

// Message represents an outbound email; HTMLBody is optional but preferred.
type Message struct {
	To       []string
	Subject  string
	HTMLBody string
	TextBody string
}

// Send delivers the message using the configured SMTP server. When SMTP is
// not configured, it logs the attempt and returns nil so the caller's
// happy-path is unaffected (this matches Laravel's "log" mail driver).
func Send(cfg Config, msg Message) error {
	if !cfg.IsEnabled() {
		log.Printf("[mailer] SMTP not configured; skipping email to %v subject=%q", msg.To, msg.Subject)
		return nil
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mailer: no recipients")
	}

	from := cfg.FromAddress
	fromHeader := from
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", cfg.FromName, from)
	}

	headers := map[string]string{
		"From":         fromHeader,
		"To":           strings.Join(msg.To, ", "),
		"Subject":      msg.Subject,
		"MIME-Version": "1.0",
	}
	body := msg.TextBody
	if msg.HTMLBody != "" {
		headers["Content-Type"] = "text/html; charset=\"UTF-8\""
		body = msg.HTMLBody
	} else {
		headers["Content-Type"] = "text/plain; charset=\"UTF-8\""
	}

	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	sb.WriteString(body)

	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	// SSL (port 465 style) requires us to dial the TLS connection ourselves
	// because net/smtp.SendMail uses STARTTLS only.
	if cfg.Encryption == "ssl" {
		tlsCfg := &tls.Config{ServerName: cfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("mailer: tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("mailer: smtp client: %w", err)
		}
		defer func() { _ = client.Quit() }()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("mailer: auth: %w", err)
			}
		}
		if err := client.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range msg.To {
			if err := client.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(sb.String())); err != nil {
			return err
		}
		return w.Close()
	}

	// Default path: plaintext or STARTTLS handled by smtp.SendMail.
	return smtp.SendMail(addr, auth, from, msg.To, []byte(sb.String()))
}
