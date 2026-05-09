package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

func (c Config) URL() string {
	vhost := c.VHost
	if vhost == "" {
		vhost = "/"
	}
	return fmt.Sprintf("amqp://%s:%s@%s:%s/%s",
		c.User, c.Password, c.Host, c.Port,
		stripLeadingSlash(vhost),
	)
}

func stripLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func (c Config) IsConfigured() bool {
	return c.Host != "" && c.User != ""
}

type Client struct {
	cfg  Config
	mu   sync.Mutex
	conn *amqp.Connection
}

func NewClient(cfg Config) (*Client, error) {
	c := &Client{cfg: cfg}
	if !cfg.IsConfigured() {
		return c, errors.New("rabbitmq: not configured")
	}
	if err := c.dial(); err != nil {
		return c, err
	}
	return c, nil
}

func (c *Client) dial() error {
	conn, err := amqp.DialConfig(c.cfg.URL(), amqp.Config{
		Heartbeat: 30 * time.Second,
		Locale:    "en_US",
		Dial:      amqp.DefaultDial(10 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("rabbitmq: dial: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) connection() (*amqp.Connection, error) {
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn, nil
	}
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c.conn, nil
}

func (c *Client) Channel() (*amqp.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, err := c.connection()
	if err != nil {
		return nil, err
	}
	return conn.Channel()
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.conn.IsClosed() {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) DeclareQueue(name string) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()
	_, err = ch.QueueDeclare(name, true, false, false, false, nil)
	return err
}

func (c *Client) Publish(ctx context.Context, queue string, body []byte) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}

	return ch.PublishWithContext(ctx,
		"",    // default exchange
		queue, // routing key == queue name
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}

type Handler func(ctx context.Context, body []byte) error

func (c *Client) Consume(ctx context.Context, queue string, handler Handler) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}

	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("rabbitmq: delivery channel closed")
			}
			if err := handler(ctx, d.Body); err != nil {
				log.Printf("[rabbitmq] handler error queue=%s: %v", queue, err)
				_ = d.Nack(false, false)
				continue
			}
			_ = d.Ack(false)
		}
	}
}
