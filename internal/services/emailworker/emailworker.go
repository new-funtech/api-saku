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
			return handle(body, mailerCfg)
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

func handle(body []byte, cfg mailer.Config) error {
	var job EmailJob
	if err := json.Unmarshal(body, &job); err != nil {
		log.Printf("[email-worker] invalid payload: %v body=%s", err, string(body))
		return nil
	}
	if len(job.To) == 0 || job.Subject == "" {
		log.Printf("[email-worker] dropping incomplete job subject=%q recipients=%d", job.Subject, len(job.To))
		return nil
	}
	if err := mailer.Send(cfg, mailer.Message{
		To:       job.To,
		Subject:  job.Subject,
		HTMLBody: job.HTMLBody,
		TextBody: job.TextBody,
	}); err != nil {
		log.Printf("[email-worker] send failed to=%v subject=%q: %v", job.To, job.Subject, err)

		return err
	}
	return nil
}
