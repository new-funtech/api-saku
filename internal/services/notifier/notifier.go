package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services/emailworker"
	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	"github.com/ganiramadhan/ganipedia/backend/pkg/mailer"
	"github.com/google/uuid"
)

const (
	roleSuperAdmin   = "super_admin"
	roleCompanyAdmin = "company_admin"
	roleBprks        = "bprks"
)

type Notifier struct {
	broker     *rabbitmq.Client
	queue      string
	userRepo   repository.UserRepository
	publishTTL time.Duration
	// direct is used when the queue is unavailable, so an email is not lost with it.
	direct mailer.Config
	// prefs lets users switch email notifications off (Pengaturan). May be nil.
	prefs EmailPreferences
}

// EmailPreferences tells which addresses belong to users who turned email notifications off.
type EmailPreferences interface {
	EmailOptedOut(ctx context.Context, emails []string) (map[string]bool, error)
}

func New(broker *rabbitmq.Client, queue string, userRepo repository.UserRepository, direct mailer.Config, prefs EmailPreferences) *Notifier {
	if queue == "" {
		queue = "email.send"
	}
	return &Notifier{
		broker:     broker,
		queue:      queue,
		userRepo:   userRepo,
		publishTTL: 5 * time.Second,
		direct:     direct,
		prefs:      prefs,
	}
}

func (n *Notifier) Send(ctx context.Context, to []string, subject, htmlBody string) {
	n.dispatch(ctx, to, subject, htmlBody)
}

func (n *Notifier) dispatch(ctx context.Context, to []string, subject, htmlBody string) {
	if n == nil {
		return
	}
	cleaned := dedupeEmails(to)
	if len(cleaned) == 0 || subject == "" {
		return
	}
	cleaned = n.withoutOptedOut(ctx, cleaned)
	if len(cleaned) == 0 {
		return
	}
	job := emailworker.EmailJob{To: cleaned, Subject: subject, HTMLBody: htmlBody}

	if n.broker == nil {
		// No queue configured: send straight away rather than silently dropping the email.
		n.sendDirect(job, "RabbitMQ is not configured")
		return
	}
	payload, err := json.Marshal(job)
	if err != nil {
		log.Printf("[notifier] marshal failed (subject=%q): %v", subject, err)
		return
	}
	pubCtx, cancel := context.WithTimeout(ctx, n.publishTTL)
	defer cancel()
	if err := n.broker.Publish(pubCtx, n.queue, payload); err != nil {
		log.Printf("[notifier] publish failed (subject=%q to=%v): %v", subject, cleaned, err)
		n.sendDirect(job, "publishing to the queue failed")
	}
}

// sendDirect delivers an email in the background over SMTP, bypassing the queue.
func (n *Notifier) sendDirect(job emailworker.EmailJob, reason string) {
	if !n.direct.IsEnabled() {
		log.Printf("[notifier] email NOT sent (%s and SMTP is not configured): to=%v subject=%q", reason, job.To, job.Subject)
		return
	}
	log.Printf("[notifier] sending directly (%s): to=%v subject=%q", reason, job.To, job.Subject)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		_ = emailworker.Deliver(ctx, n.direct, job)
	}()
}

func (n *Notifier) AdminCompanyUsers(ctx context.Context, bujpID *uuid.UUID) []model.User {
	if n == nil || n.userRepo == nil || bujpID == nil || *bujpID == uuid.Nil {
		return nil
	}
	users, _, err := n.userRepo.FindAll(ctx, 1, 999, map[string]interface{}{
		"role":    roleCompanyAdmin,
		"bujp_id": *bujpID,
		"status":  "active",
	})
	if err != nil {
		log.Printf("[notifier] AdminCompanyUsers lookup failed: %v", err)
		return nil
	}
	return users
}

func (n *Notifier) AdminPusatUsers(ctx context.Context) []model.User {
	if n == nil || n.userRepo == nil {
		return nil
	}
	users, _, err := n.userRepo.FindAll(ctx, 1, 999, map[string]interface{}{
		"role":   roleSuperAdmin,
		"status": "active",
	})
	if err != nil {
		log.Printf("[notifier] AdminPusatUsers lookup failed: %v", err)
		return nil
	}
	return users
}

func (n *Notifier) AdminBprksUsers(ctx context.Context) []model.User {
	if n == nil || n.userRepo == nil {
		return nil
	}
	users, _, err := n.userRepo.FindAll(ctx, 1, 999, map[string]interface{}{
		"role":   roleBprks,
		"status": "active",
	})
	if err != nil {
		log.Printf("[notifier] AdminBprksUsers lookup failed: %v", err)
		return nil
	}
	return users
}

func userEmails(users []model.User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		if u.Email != "" {
			out = append(out, u.Email)
		}
	}
	return out
}

func (n *Notifier) adminCompanyEmails(ctx context.Context, bujpID *uuid.UUID) []string {
	return userEmails(n.AdminCompanyUsers(ctx, bujpID))
}

func (n *Notifier) adminPusatEmails(ctx context.Context) []string {
	return userEmails(n.AdminPusatUsers(ctx))
}

func (n *Notifier) adminBprksEmails(ctx context.Context) []string {
	return userEmails(n.AdminBprksUsers(ctx))
}

func personnelEmail(p *model.Personnel) []string {
	if p == nil || p.Email == nil {
		return nil
	}
	e := strings.TrimSpace(*p.Email)
	if e == "" {
		return nil
	}
	return []string{e}
}

func dedupeEmails(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.TrimSpace(strings.ToLower(e))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func formatRupiah(amount float64) string {
	whole := int64(amount)
	neg := whole < 0
	if neg {
		whole = -whole
	}
	s := fmt.Sprintf("%d", whole)
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	lead := n % 3
	if lead > 0 {
		b.WriteString(s[:lead])
		if n > lead {
			b.WriteByte('.')
		}
	}
	for i := lead; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteByte('.')
		}
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func formatDateID(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	weekdays := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	months := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	return fmt.Sprintf("%s, %d %s %d", weekdays[int(t.Weekday())], t.Day(), months[int(t.Month())-1], t.Year())
}

func formatTimeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"15:04:05",
		"15:04",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("15:04") + " WIB"
		}
	}
	return s
}

// withoutOptedOut drops the recipients who turned email notifications off. If the lookup fails the
// email goes out anyway: a missed setting must not swallow a notification.
func (n *Notifier) withoutOptedOut(ctx context.Context, emails []string) []string {
	if n.prefs == nil {
		return emails
	}
	off, err := n.prefs.EmailOptedOut(ctx, emails)
	if err != nil {
		log.Printf("[notifier] could not read email preferences: %v", err)
		return emails
	}
	if len(off) == 0 {
		return emails
	}
	kept := make([]string, 0, len(emails))
	for _, e := range emails {
		if !off[strings.ToLower(strings.TrimSpace(e))] {
			kept = append(kept, e)
		}
	}
	return kept
}
