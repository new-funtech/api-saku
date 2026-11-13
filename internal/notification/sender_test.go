package notification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
)

type fakeStore struct {
	mu      sync.Mutex
	tokens  []string
	deleted []string
	pushOff bool
}

func (f *fakeStore) PushEnabled(context.Context, uuid.UUID) bool               { return !f.pushOff }
func (f *fakeStore) TokensByUser(context.Context, uuid.UUID) ([]string, error) { return f.tokens, nil }
func (f *fakeStore) DeleteTokens(_ context.Context, t []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, t...)
	return nil
}

type fakeMessenger struct {
	mu      sync.Mutex
	batches [][]string
	topic   *messaging.Message
	// perToken decides the answer for a token; nil means success.
	perToken map[string]error
	done     chan struct{}
}

func (f *fakeMessenger) SendEachForMulticast(_ context.Context, m *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	f.mu.Lock()
	f.batches = append(f.batches, m.Tokens)
	f.mu.Unlock()
	resp := &messaging.BatchResponse{}
	for _, tok := range m.Tokens {
		if err := f.perToken[tok]; err != nil {
			resp.FailureCount++
			resp.Responses = append(resp.Responses, &messaging.SendResponse{Error: err})
		} else {
			resp.SuccessCount++
			resp.Responses = append(resp.Responses, &messaging.SendResponse{Success: true})
		}
	}
	if f.done != nil {
		select {
		case f.done <- struct{}{}:
		default:
		}
	}
	return resp, nil
}

func (f *fakeMessenger) Send(_ context.Context, m *messaging.Message) (string, error) {
	f.topic = m
	return "id", nil
}

// The SDK's error type cannot be built outside it, so the classifiers are replaced by ones that
// read a code prefix from a plain error. The SDK's real classifiers are used in production.
func TestMain(m *testing.M) {
	isUnregistered = func(err error) bool { return strings.HasPrefix(err.Error(), "UNREGISTERED") }
	isInvalidArgument = func(err error) bool { return strings.HasPrefix(err.Error(), "INVALID_ARGUMENT") }
	os.Exit(m.Run())
}

func unregistered() error { return errors.New("UNREGISTERED: Requested entity was not found.") }
func badToken() error {
	return errors.New("INVALID_ARGUMENT: The registration token is not a valid FCM registration token")
}
func badPayload() error { return errors.New("INVALID_ARGUMENT: Invalid JSON payload received.") }

func tokens(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("token-%04d", i)
	}
	return out
}

func TestSendToUserRemovesDeadTokensOnly(t *testing.T) {
	store := &fakeStore{tokens: []string{"good", "gone", "malformed", "payload-problem"}}
	m := &fakeMessenger{perToken: map[string]error{
		"gone":            unregistered(),
		"malformed":       badToken(),
		"payload-problem": badPayload(), // not about the token: must be kept
	}}
	s := NewSender(m, store)

	res, err := s.SendToUser(context.Background(), uuid.New(), "Cuti disetujui", "Pengajuan cuti Anda disetujui.", map[string]string{"type": "leave", "id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 1 || res.Failed != 3 || res.Removed != 2 {
		t.Fatalf("result = %+v", res)
	}
	got := map[string]bool{}
	for _, d := range store.deleted {
		got[d] = true
	}
	if !got["gone"] || !got["malformed"] || got["good"] || got["payload-problem"] {
		t.Fatalf("deleted = %v", store.deleted)
	}
}

func TestSendToUserBatchesAtFiveHundred(t *testing.T) {
	store := &fakeStore{tokens: tokens(1200)}
	m := &fakeMessenger{}
	s := NewSender(m, store)
	res, err := s.SendToUser(context.Background(), uuid.New(), "t", "b", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.batches) != 3 || len(m.batches[0]) != 500 || len(m.batches[1]) != 500 || len(m.batches[2]) != 200 {
		t.Fatalf("batch sizes = %d", len(m.batches))
	}
	if res.Sent != 1200 {
		t.Fatalf("sent = %d", res.Sent)
	}
}

func TestSendToUserWithoutTokensDoesNotCallFCM(t *testing.T) {
	m := &fakeMessenger{}
	s := NewSender(m, &fakeStore{})
	if _, err := s.SendToUser(context.Background(), uuid.New(), "t", "b", nil); err != nil {
		t.Fatal(err)
	}
	if len(m.batches) != 0 {
		t.Fatal("FCM was called without tokens")
	}
}

func TestOnlyTypeAndIDAreSent(t *testing.T) {
	got := cleanData(map[string]string{"type": "payslip", "id": "9", "net_salary": "5000000", "amount": "x"})
	if len(got) != 2 || got["type"] != "payslip" || got["id"] != "9" {
		t.Fatalf("data = %v", got)
	}
}

func TestTitleAndBodyAreClipped(t *testing.T) {
	if got := clip(string(make([]rune, 500)), 100); len([]rune(got)) != 100 {
		t.Fatalf("len = %d", len([]rune(got)))
	}
	if clip("  halo  ", 100) != "halo" {
		t.Fatal("not trimmed")
	}
}

func TestSendToTopic(t *testing.T) {
	m := &fakeMessenger{}
	s := NewSender(m, &fakeStore{})
	if err := s.SendToTopic(context.Background(), "/topics/branch-bandung", "Pengumuman", "Libur bersama", map[string]string{"type": "announcement", "id": "7", "secret": "x"}); err != nil {
		t.Fatal(err)
	}
	if m.topic == nil || m.topic.Topic != "branch-bandung" || len(m.topic.Data) != 2 {
		t.Fatalf("message = %+v", m.topic)
	}
	for _, bad := range []string{"", "has space", "a/b", "x@y"} {
		if err := s.SendToTopic(context.Background(), bad, "t", "b", nil); err == nil {
			t.Errorf("topic %q should be rejected", bad)
		}
	}
}

func TestDisabledSenderIsANoOp(t *testing.T) {
	s := NewSender(nil, &fakeStore{tokens: []string{"x"}})
	if s.Enabled() {
		t.Fatal("must be disabled without a messenger")
	}
	if _, err := s.SendToUser(context.Background(), uuid.New(), "t", "b", nil); err != nil {
		t.Fatal(err)
	}
	s.SendToUserAsync(uuid.New(), "t", "b", nil) // must not panic or block
	var nilSender *Sender
	nilSender.Start(context.Background())
	nilSender.Stop()
}

func TestAsyncSendRunsInTheBackground(t *testing.T) {
	m := &fakeMessenger{done: make(chan struct{}, 1)}
	s := NewSender(m, &fakeStore{tokens: []string{"a"}})
	s.Start(context.Background())
	defer s.Stop()

	start := time.Now()
	s.SendToUserAsync(uuid.New(), "t", "b", nil)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("enqueue blocked")
	}
	select {
	case <-m.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the worker never sent the push")
	}
}

func TestMulticastErrorIsReturned(t *testing.T) {
	s := NewSender(errMessenger{}, &fakeStore{tokens: []string{"a"}})
	if _, err := s.SendToUser(context.Background(), uuid.New(), "t", "b", nil); err == nil {
		t.Fatal("expected the FCM error")
	}
}

type errMessenger struct{}

func (errMessenger) SendEachForMulticast(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	return nil, errors.New("boom")
}
func (errMessenger) Send(context.Context, *messaging.Message) (string, error) { return "", nil }

func TestPushIsSkippedWhenTheUserTurnedItOff(t *testing.T) {
	m := &fakeMessenger{}
	s := NewSender(m, &fakeStore{tokens: []string{"a"}, pushOff: true})
	res, err := s.SendToUser(context.Background(), uuid.New(), "t", "b", nil)
	if err != nil || res.Sent != 0 || len(m.batches) != 0 {
		t.Fatalf("res=%+v err=%v batches=%d: nothing may be sent", res, err, len(m.batches))
	}
}
