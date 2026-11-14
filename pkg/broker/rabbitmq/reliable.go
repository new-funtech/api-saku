package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Reliable messaging on top of Client: publisher confirms, a retry ladder with a dead-letter queue,
// and a batching consumer with manual acks. Used by the attendance queue; the email queue keeps
// using Publish/Consume.

// ErrNotConfirmed means the broker did not accept the message. The caller must treat the publish as
// failed (and tell its own client), never assume it was queued.
var ErrNotConfirmed = errors.New("rabbitmq: message not confirmed by the broker")

const (
	// HeaderAttempt counts how many times a message has been processed and failed.
	HeaderAttempt = "x-attempt"
	// HeaderError holds the last error when a message is parked in the dead-letter queue.
	HeaderError = "x-error"
)

// Topology names the queues of one logical queue: <name>, <name>.retry.<i> and <name>.dlq.
type Topology struct {
	Queue       string
	RetryDelays []time.Duration // one retry queue per delay; a message that fails n times waits RetryDelays[n-1]
}

func (t Topology) DLQ() string             { return t.Queue + ".dlq" }
func (t Topology) RetryQueue(i int) string { return fmt.Sprintf("%s.retry.%d", t.Queue, i) }

// Declare creates the durable queues. Retry queues expire messages back into the main queue
// (dead-letter to the default exchange), which is what gives the backoff.
func (c *Client) Declare(t Topology) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if _, err := ch.QueueDeclare(t.DLQ(), true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(t.Queue, true, false, false, false, nil); err != nil {
		return err
	}
	for i, d := range t.RetryDelays {
		_, err := ch.QueueDeclare(t.RetryQueue(i), true, false, false, false, amqp.Table{
			"x-message-ttl":             int64(d / time.Millisecond),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": t.Queue,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// QueueDepth is the number of ready messages in a queue (passive declare, nothing is created).
func (c *Client) QueueDepth(queue string) (int, error) {
	ch, err := c.Channel()
	if err != nil {
		return 0, err
	}
	defer func() { _ = ch.Close() }()
	q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
	if err != nil {
		return 0, err
	}
	return q.Messages, nil
}

// ErrUnroutable means the broker confirmed the message but had no queue to put it in (the queue
// does not exist), so it was dropped. Publishing is mandatory so this is never silent.
var ErrUnroutable = errors.New("rabbitmq: message could not be routed to a queue")

type pubChannel struct {
	ch      *amqp.Channel
	returns chan amqp.Return
}

// Publisher publishes with confirms over a small pool of channels.
type Publisher struct {
	client *Client
	pool   chan *pubChannel
}

// NewPublisher opens up to `size` confirm-mode channels lazily.
func (c *Client) NewPublisher(size int) *Publisher {
	if size < 1 {
		size = 4
	}
	p := &Publisher{client: c, pool: make(chan *pubChannel, size)}
	for i := 0; i < size; i++ {
		p.pool <- nil // a nil slot is opened on first use
	}
	return p
}

func (p *Publisher) open() (*pubChannel, error) {
	ch, err := p.client.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &pubChannel{ch: ch, returns: ch.NotifyReturn(make(chan amqp.Return, 8))}, nil
}

// Publish sends a persistent, mandatory message and waits for the broker's confirm. It returns an
// error if the broker does not confirm within ctx, or confirms but cannot route it.
func (p *Publisher) Publish(ctx context.Context, queue string, body []byte, headers amqp.Table) error {
	var pc *pubChannel
	select {
	case pc = <-p.pool:
	case <-ctx.Done():
		return ctx.Err()
	}
	healthy := false
	defer func() {
		if !healthy && pc != nil {
			_ = pc.ch.Close()
			pc = nil
		}
		p.pool <- pc
	}()

	if pc == nil || pc.ch.IsClosed() {
		var err error
		if pc, err = p.open(); err != nil {
			return err
		}
	}
	conf, err := pc.ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Headers:      headers,
		Body:         body,
	})
	if err != nil {
		return err
	}
	acked, err := conf.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return ErrNotConfirmed
	}
	healthy = true
	select {
	case <-pc.returns: // the broker returns an unroutable message before it acks
		return ErrUnroutable
	default:
	}
	return nil
}

// BatchHandler gets up to maxBatch deliveries. It must Ack or Nack every one of them.
type BatchHandler func(ctx context.Context, batch []amqp.Delivery)

// ConsumeBatches reads with manual acks and a prefetch limit, and hands the handler batches of at
// most maxBatch messages or whatever arrived within maxWait. When ctx is cancelled it finishes the
// batch in hand (the handler gets a context that is NOT cancelled) and returns, so nothing in
// flight is lost; unacked messages go back to the queue when the channel closes.
func (c *Client) ConsumeBatches(ctx context.Context, queue string, prefetch, maxBatch int, maxWait time.Duration, h BatchHandler) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if prefetch < maxBatch {
		prefetch = maxBatch
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	closed := ch.NotifyClose(make(chan *amqp.Error, 1))

	batch := make([]amqp.Delivery, 0, maxBatch)
	timer := time.NewTimer(maxWait)
	defer timer.Stop()
	flush := func() {
		if len(batch) > 0 {
			h(context.WithoutCancel(ctx), batch)
			batch = make([]amqp.Delivery, 0, maxBatch)
		}
	}
	for {
		select {
		case <-ctx.Done():
			flush()
			return nil
		case e := <-closed:
			flush()
			if e == nil {
				return errors.New("rabbitmq: channel closed")
			}
			return e
		case d, ok := <-deliveries:
			if !ok {
				flush()
				return errors.New("rabbitmq: delivery channel closed")
			}
			if len(batch) == 0 {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(maxWait)
			}
			batch = append(batch, d)
			if len(batch) >= maxBatch {
				flush()
			}
		case <-timer.C:
			flush()
			timer.Reset(maxWait)
		}
	}
}

// Requeue moves up to limit messages from one queue to another with confirms (used to reprocess the
// dead-letter queue). It returns how many were moved. Each message is acked only after the confirm.
func (c *Client) Requeue(ctx context.Context, from, to string, limit int, pub *Publisher) (int, error) {
	ch, err := c.Channel()
	if err != nil {
		return 0, err
	}
	defer func() { _ = ch.Close() }()
	moved := 0
	for limit <= 0 || moved < limit {
		d, ok, err := ch.Get(from, false)
		if err != nil {
			return moved, err
		}
		if !ok {
			break
		}
		headers := amqp.Table{}
		for k, v := range d.Headers {
			if k != HeaderAttempt && k != HeaderError {
				headers[k] = v
			}
		}
		if err := pub.Publish(ctx, to, d.Body, headers); err != nil {
			_ = d.Nack(false, true)
			return moved, err
		}
		_ = d.Ack(false)
		moved++
	}
	log.Printf("[rabbitmq] requeued %d messages %s -> %s", moved, from, to)
	return moved, nil
}
