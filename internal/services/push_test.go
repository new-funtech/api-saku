package services

import (
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

type recordedPush struct {
	user        uuid.UUID
	title, body string
	data        map[string]string
}

type fakePush struct{ got []recordedPush }

func (f *fakePush) SendToUserAsync(user uuid.UUID, title, body string, data map[string]string) {
	f.got = append(f.got, recordedPush{user, title, body, data})
}

func TestPushBodyKeepsEveryLine(t *testing.T) {
	if got := pushBody("Pengajuan pinjaman Anda ditolak.\nAlasan: data tidak lengkap"); got != "Pengajuan pinjaman Anda ditolak.\nAlasan: data tidak lengkap" {
		t.Fatalf("body = %q", got)
	}
}

func TestNewNotificationsAlsoGoOutAsPush(t *testing.T) {
	push := &fakePush{}
	s := &notificationServiceImpl{push: push} // no websocket hub: only the push path
	user, loan := uuid.New(), uuid.New()
	s.PushRealtime([]model.Notification{{
		RecipientUserID: user, Type: model.NotificationTypeLoanRejected,
		Title: "Pengajuan pinjaman ditolak", Body: "Pengajuan pinjaman Anda ditolak.\nAlasan: rahasia", EntityID: loan,
	}})
	if len(push.got) != 1 {
		t.Fatalf("pushes = %d", len(push.got))
	}
	p := push.got[0]
	if p.user != user || p.body != "Pengajuan pinjaman Anda ditolak.\nAlasan: rahasia" {
		t.Fatalf("push = %+v", p)
	}
	if len(p.data) != 2 || p.data["type"] != "LOAN_REJECTED" || p.data["id"] != loan.String() {
		t.Fatalf("data = %v (only type and id may be sent)", p.data)
	}
}
