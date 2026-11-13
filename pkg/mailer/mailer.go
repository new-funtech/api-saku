// Package mailer sends transactional emails over SMTP using only the standard library. It is a
// no-op (with a log line) when SMTP is not configured, so callers can fire and forget.
package mailer

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
)

const (
	dialTimeout = 15 * time.Second
	sendTimeout = 60 * time.Second
)

// Config captures the SMTP credentials and sender identity loaded from env.
type Config struct {
	Host        string
	Port        string
	Username    string
	Password    string
	Encryption  string // see Mode
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
		Encryption:  strings.ToLower(strings.TrimSpace(config.GetEnv("MAIL_ENCRYPTION", "tls"))),
		FromAddress: config.GetEnv("MAIL_FROM_ADDRESS", ""),
		FromName:    config.GetEnv("MAIL_FROM_NAME", "HRMIS"),
	}
}

// IsEnabled reports whether the minimum SMTP fields are present.
func (c Config) IsEnabled() bool {
	return c.Host != "" && c.FromAddress != ""
}

// Security is how the connection to the SMTP server is protected.
type Security string

const (
	// SecurityImplicitTLS wraps the whole connection in TLS from the start (port 465).
	SecurityImplicitTLS Security = "ssl"
	// SecurityStartTLS starts in plain text and upgrades with STARTTLS (port 587).
	SecurityStartTLS Security = "starttls"
	// SecurityNone sends in plain text (local relays only).
	SecurityNone Security = "none"
)

// Mode works out the security mode from MAIL_ENCRYPTION and the port. The setting is accepted in
// the usual spellings ("ssl", "true", "tls", "starttls", ...), and port 465 always means implicit
// TLS: a "true" on port 465 used to fall through to STARTTLS, which hangs against an implicit-TLS
// server and made every email fail with EOF.
func (c Config) Mode() Security {
	switch strings.ToLower(strings.TrimSpace(c.Encryption)) {
	case "ssl", "smtps", "implicit", "true", "1", "yes", "on":
		return SecurityImplicitTLS
	case "none", "false", "0", "no", "off":
		return SecurityNone
	case "tls", "starttls":
		if c.Port == "465" {
			return SecurityImplicitTLS
		}
		return SecurityStartTLS
	}
	if c.Port == "465" {
		return SecurityImplicitTLS
	}
	return SecurityStartTLS
}

// Message represents an outbound email; HTMLBody is optional but preferred.
type Message struct {
	To       []string
	Subject  string
	HTMLBody string
	TextBody string
}

// Send delivers the message. When SMTP is not configured it logs and returns nil.
func Send(cfg Config, msg Message) error {
	if !cfg.IsEnabled() {
		log.Printf("[mailer] SMTP not configured; skipping email to %v subject=%q", msg.To, msg.Subject)
		return nil
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mailer: no recipients")
	}

	data := buildMessage(cfg, msg)
	client, err := dial(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return fmt.Errorf("mailer: auth: %w", err)
			}
		}
	}
	if err := client.Mail(cfg.FromAddress); err != nil {
		return fmt.Errorf("mailer: sender rejected: %w", err)
	}
	for _, rcpt := range msg.To {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mailer: recipient %s rejected: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: finish: %w", err)
	}
	_ = client.Quit()
	return nil
}

// dial opens the SMTP session with timeouts, wrapping it in TLS the way the server expects.
func dial(cfg Config) (*smtp.Client, error) {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("mailer: connect %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(sendTimeout))

	mode := cfg.Mode()
	if mode == SecurityImplicitTLS {
		conn = tls.Client(conn, tlsCfg)
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mailer: smtp handshake (%s on %s): %w", mode, addr, err)
	}
	if mode == SecurityStartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("mailer: starttls: %w", err)
			}
		}
	}
	return client, nil
}

// buildMessage renders a standards-compliant message: encoded subject and sender name (they may
// contain non-ASCII such as an em dash), Date and Message-ID headers, and a base64 body wrapped
// at 76 columns (long HTML lines are rejected by many servers).
func buildMessage(cfg Config, msg Message) []byte {
	from := mail.Address{Name: cfg.FromName, Address: cfg.FromAddress}
	contentType := "text/plain; charset=\"UTF-8\""
	body := msg.TextBody
	if msg.HTMLBody != "" {
		contentType = "text/html; charset=\"UTF-8\""
		body = msg.HTMLBody
	}

	domain := "localhost"
	if at := strings.LastIndex(cfg.FromAddress, "@"); at >= 0 && at < len(cfg.FromAddress)-1 {
		domain = cfg.FromAddress[at+1:]
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)

	var sb strings.Builder
	header := func(k, v string) { sb.WriteString(k + ": " + v + "\r\n") }
	header("From", from.String())
	header("To", strings.Join(msg.To, ", "))
	header("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", contentType)
	header("Content-Transfer-Encoding", "base64")
	sb.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		sb.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	sb.WriteString(encoded + "\r\n")
	return []byte(sb.String())
}
