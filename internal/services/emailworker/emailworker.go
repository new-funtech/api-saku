package emailworker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	"github.com/ganiramadhan/ganipedia/backend/pkg/mailer"
)

type EmailJob struct {
	To       []string `json:"to"`
	Subject  string   `json:"subject"`
	HTMLBody string   `json:"html_body,omitempty"`
	TextBody string   `json:"text_body,omitempty"`
}

func Run(ctx context.Context, client *rabbitmq.Client, queue string, mailerCfg mailer.Config) {
	if client == nil {
		log.Println("[email-worker] disabled: no broker client")
		return
	}
	if queue == "" {
		queue = "email.send"
	}
	log.Printf("[email-worker] starting on queue=%s", queue)

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		err := client.Consume(ctx, queue, func(ctx context.Context, body []byte) error {
			return handle(ctx, body, mailerCfg)
		})
		if errors.Is(err, context.Canceled) {
			log.Println("[email-worker] shutdown")
			return
		}
		log.Printf("[email-worker] consume ended: %v (retry in %s)", err, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		// Exponential backoff capped at maxBackoff.
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// maxSendAttempts / retryBackoff — bounded in-process retry for a single
// email job before it's given up on.
//
// PENTING (root cause "email hilang begitu gagal sekali"): rabbitmq.Consume
// calls d.Nack(false, false) — requeue=false — on ANY handler error, so a
// single transient failure (e.g. the SMTP server closing the connection
// mid-handshake → "EOF", a common symptom of a brief network hiccup or the
// server hanging up on a slow/idle connection) permanently discards that
// email with zero retry; there's also no dead-letter queue to recover it
// from afterward. Retrying entirely at the RabbitMQ level (requeue=true)
// risks an infinite reprocessing loop for a job that's genuinely
// unsendable (e.g. bad recipient) since there's no attempt-count tracked in
// message headers. The safest fix that needs no protocol/header changes:
// retry the SMTP send itself a bounded number of times, right here, before
// letting the message be Nack'd for good.
const (
	maxSendAttempts  = 3
	retryBackoffStep = 2 * time.Second
)

func handle(ctx context.Context, body []byte, cfg mailer.Config) error {
	var job EmailJob
	if err := json.Unmarshal(body, &job); err != nil {
		log.Printf("[email-worker] invalid payload: %v body=%s", err, string(body))
		return nil
	}
	if len(job.To) == 0 || job.Subject == "" {
		log.Printf("[email-worker] dropping incomplete job subject=%q recipients=%d", job.Subject, len(job.To))
		return nil
	}

	msg := mailer.Message{
		To:       job.To,
		Subject:  job.Subject,
		HTMLBody: job.HTMLBody,
		TextBody: job.TextBody,
	}

	var lastErr error
	for attempt := 1; attempt <= maxSendAttempts; attempt++ {
		lastErr = mailer.Send(cfg, msg)
		if lastErr == nil {
			return nil
		}
		log.Printf("[email-worker] send attempt %d/%d failed to=%v subject=%q: %v", attempt, maxSendAttempts, job.To, job.Subject, lastErr)
		if attempt == maxSendAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * retryBackoffStep):
		}
	}
	log.Printf("[email-worker] giving up after %d attempts to=%v subject=%q: %v", maxSendAttempts, job.To, job.Subject, lastErr)
	return lastErr
}
