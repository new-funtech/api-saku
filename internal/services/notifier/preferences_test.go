package notifier

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakePrefs struct {
	off map[string]bool
	err error
}

func (f fakePrefs) EmailOptedOut(context.Context, []string) (map[string]bool, error) {
	return f.off, f.err
}

func TestEmailsToUsersWhoOptedOutAreDropped(t *testing.T) {
	n := &Notifier{prefs: fakePrefs{off: map[string]bool{"quiet@example.com": true}}}
	got := n.withoutOptedOut(context.Background(), []string{"Quiet@Example.com", "loud@example.com"})
	if !reflect.DeepEqual(got, []string{"loud@example.com"}) {
		t.Fatalf("got %v", got)
	}
}

func TestEveryoneIsKeptWithoutPreferencesOrOnAnError(t *testing.T) {
	all := []string{"a@example.com", "b@example.com"}
	if got := (&Notifier{}).withoutOptedOut(context.Background(), all); !reflect.DeepEqual(got, all) {
		t.Fatalf("no prefs: %v", got)
	}
	n := &Notifier{prefs: fakePrefs{err: errors.New("db down")}}
	if got := n.withoutOptedOut(context.Background(), all); !reflect.DeepEqual(got, all) {
		t.Fatalf("a failed lookup must not swallow emails: %v", got)
	}
}
