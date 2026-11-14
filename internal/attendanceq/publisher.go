package attendanceq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher is the Enqueuer backed by RabbitMQ with publisher confirms.
type Publisher struct {
	client   *rabbitmq.Client
	pub      *rabbitmq.Publisher
	cfg      Config
	mu       sync.Mutex
	declared bool
}

// NewPublisher opens the confirm channels lazily. The queues are declared (idempotently) on first
// use, so the API can start while the broker is still coming up.
func NewPublisher(client *rabbitmq.Client, cfg Config) *Publisher {
	return &Publisher{client: client, pub: client.NewPublisher(cfg.PublishPool), cfg: cfg}
}

func (p *Publisher) ensureDeclared() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.declared {
		return nil
	}
	if err := p.client.Declare(Topology(p.cfg)); err != nil {
		return fmt.Errorf("attendance queue: declare: %w", err)
	}
	p.declared = true
	return nil
}

// Topology maps the config to the broker's queue layout.
func Topology(cfg Config) rabbitmq.Topology {
	return rabbitmq.Topology{Queue: cfg.Queue, RetryDelays: cfg.RetryDelays}
}

func (p *Publisher) Enqueue(ctx context.Context, ev Event) error {
	if err := p.ensureDeclared(); err != nil {
		return err
	}
	ev.Version = 1
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, p.cfg.PublishWait)
	defer cancel()
	return p.pub.Publish(ctx, p.cfg.Queue, body, amqp.Table{rabbitmq.HeaderAttempt: int32(0)})
}
