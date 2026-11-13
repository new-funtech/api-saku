package notification

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
)

const (
	// maxBatch is the most tokens FCM accepts in one multicast call.
	maxBatch = 500

	maxTitleLen = 100
	maxBodyLen  = 300

	queueSize   = 256
	workerCount = 4
	jobTimeout  = 30 * time.Second
)

// TokenStore is where the sender reads a user's device tokens and drops the dead ones.
type TokenStore interface {
	TokensByUser(ctx context.Context, userID uuid.UUID) ([]string, error)
	DeleteTokens(ctx context.Context, tokens []string) error
	// PushEnabled reports whether the user wants push notifications (the setting in Pengaturan).
	PushEnabled(ctx context.Context, userID uuid.UUID) bool
}

// allowedData are the only keys a push may carry. The payload is a pointer for the app to open the
// right screen (type + id); anything sensitive (amounts, salaries) stays out of title, body and data.
var allowedData = map[string]bool{"type": true, "id": true}

// The SDK's own classifiers; variables so tests can stand in (its error type cannot be built
// outside the SDK).
var (
	isUnregistered    = messaging.IsUnregistered
	isInvalidArgument = messaging.IsInvalidArgument
)

var topicPattern = regexp.MustCompile(`^[a-zA-Z0-9-_.~%]{1,900}$`)

// Result summarizes one send.
type Result struct {
	Sent    int
	Failed  int
	Removed int // dead tokens deleted from the store
}

// Sender delivers notifications. Use SendToUser / SendToTopic from code that can wait, or the
// *Async variants from request handlers, which only enqueue the work.
type Sender struct {
	messenger Messenger // nil: disabled
	store     TokenStore

	jobs chan func(context.Context)
	wg   sync.WaitGroup
	once sync.Once
	stop context.CancelFunc
}

// NewSender returns a sender. A nil messenger disables it.
func NewSender(messenger Messenger, store TokenStore) *Sender {
	return &Sender{messenger: messenger, store: store, jobs: make(chan func(context.Context), queueSize)}
}

// Enabled reports whether pushes are actually delivered.
func (s *Sender) Enabled() bool { return s != nil && s.messenger != nil }

// Start launches the background workers behind the *Async methods. Call Stop on shutdown.
func (s *Sender) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.once.Do(func() {
		ctx, s.stop = context.WithCancel(ctx)
		for i := 0; i < workerCount; i++ {
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case job := <-s.jobs:
						s.run(ctx, job)
					}
				}
			}()
		}
	})
}

// Stop finishes the workers. Jobs still queued are dropped.
func (s *Sender) Stop() {
	if s == nil || s.stop == nil {
		return
	}
	s.stop()
	s.wg.Wait()
}

// run executes one job with a timeout, and a panic in it must not take a worker down.
func (s *Sender) run(parent context.Context, job func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[push] job panicked: %v", r)
		}
	}()
	ctx, cancel := context.WithTimeout(parent, jobTimeout)
	defer cancel()
	job(ctx)
}

func (s *Sender) enqueue(job func(context.Context)) {
	if !s.Enabled() {
		return
	}
	select {
	case s.jobs <- job:
	default:
		log.Println("[push] queue full, notification dropped")
	}
}

// SendToUserAsync queues a push to all of the user's devices and returns at once.
func (s *Sender) SendToUserAsync(userID uuid.UUID, title, body string, data map[string]string) {
	s.enqueue(func(ctx context.Context) {
		if _, err := s.SendToUser(ctx, userID, title, body, data); err != nil {
			log.Printf("[push] user %s: %v", userID, err)
		}
	})
}

// SendToTopicAsync queues a push to a topic and returns at once.
func (s *Sender) SendToTopicAsync(topic, title, body string, data map[string]string) {
	s.enqueue(func(ctx context.Context) {
		if err := s.SendToTopic(ctx, topic, title, body, data); err != nil {
			log.Printf("[push] topic %s: %v", topic, err)
		}
	})
}

// SendToUser pushes to every device of the user, in batches of at most 500, and deletes the tokens
// FCM reports as unregistered or invalid.
func (s *Sender) SendToUser(ctx context.Context, userID uuid.UUID, title, body string, data map[string]string) (Result, error) {
	var res Result
	if !s.Enabled() {
		return res, nil
	}
	if !s.store.PushEnabled(ctx, userID) {
		return res, nil // the user switched push notifications off
	}
	tokens, err := s.store.TokensByUser(ctx, userID)
	if err != nil {
		return res, fmt.Errorf("load tokens: %w", err)
	}
	if len(tokens) == 0 {
		return res, nil
	}
	payload := cleanData(data)
	title, body = clip(title, maxTitleLen), clip(body, maxBodyLen)

	var dead []string
	for start := 0; start < len(tokens); start += maxBatch {
		end := min(start+maxBatch, len(tokens))
		batch := tokens[start:end]
		resp, err := s.messenger.SendEachForMulticast(ctx, &messaging.MulticastMessage{
			Tokens:       batch,
			Notification: &messaging.Notification{Title: title, Body: body},
			Data:         payload,
			Android:      androidConfig(payload),
		})
		if err != nil {
			return res, fmt.Errorf("fcm multicast: %w", err)
		}
		res.Sent += resp.SuccessCount
		res.Failed += resp.FailureCount
		dead = append(dead, invalidTokens(batch, resp)...)
	}
	if len(dead) > 0 {
		if err := s.store.DeleteTokens(ctx, dead); err != nil {
			return res, fmt.Errorf("remove dead tokens: %w", err)
		}
		res.Removed = len(dead)
	}
	return res, nil
}

// SendToTopic pushes to everyone subscribed to the topic (e.g. "branch-bandung", "division-ops").
func (s *Sender) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) error {
	if !s.Enabled() {
		return nil
	}
	topic = strings.TrimPrefix(strings.TrimSpace(topic), "/topics/")
	if !topicPattern.MatchString(topic) {
		return fmt.Errorf("invalid topic %q", topic)
	}
	_, err := s.messenger.Send(ctx, &messaging.Message{
		Topic:        topic,
		Notification: &messaging.Notification{Title: clip(title, maxTitleLen), Body: clip(body, maxBodyLen)},
		Data:         cleanData(data),
		Android:      androidConfig(cleanData(data)),
	})
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	return nil
}

// invalidTokens picks the tokens of a batch that FCM says are dead. INVALID_ARGUMENT counts only
// when it is about the token itself: it is also what a malformed payload returns, and that must
// not wipe a user's working tokens.
func invalidTokens(batch []string, resp *messaging.BatchResponse) []string {
	var dead []string
	for i, r := range resp.Responses {
		if i >= len(batch) || r.Success || r.Error == nil {
			continue
		}
		if isUnregistered(r.Error) ||
			(isInvalidArgument(r.Error) && strings.Contains(strings.ToLower(r.Error.Error()), "registration token")) {
			dead = append(dead, batch[i])
		}
	}
	return dead
}

// cleanData keeps only the allowed keys (type, id), so nothing sensitive rides along by accident.
func cleanData(in map[string]string) map[string]string {
	out := make(map[string]string, len(allowedData))
	for k, v := range in {
		if !allowedData[k] {
			log.Printf("[push] dropped data key %q: only type and id are sent", k)
			continue
		}
		out[k] = v
	}
	return out
}

func clip(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// androidConfig styles the notification: the app's blue accent, its own channel, and a tag per
// item so a repeated update replaces the earlier notification instead of stacking.
func androidConfig(data map[string]string) *messaging.AndroidConfig {
	n := &messaging.AndroidNotification{
		Color:     "#1D4ED8",
		ChannelID: "saku_updates",
		Priority:  messaging.PriorityHigh,
	}
	if id := data["id"]; id != "" {
		n.Tag = id
	}
	return &messaging.AndroidConfig{Priority: "high", Notification: n}
}
