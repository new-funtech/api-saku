package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/attendanceq"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeEnqueuer struct {
	events []attendanceq.Event
	err    error
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, ev attendanceq.Event) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, ev)
	return nil
}

func asyncService(a *model.Assignment) (*attendanceServiceImpl, *fakeEnqueuer, *fakeAttendanceRepo) {
	svc, repo, _ := newClockService(a)
	enq := &fakeEnqueuer{}
	svc.SetEnqueuer(enq)
	return svc, enq, repo
}

func TestAsyncIsOffByDefaultAndNeedsAPublisher(t *testing.T) {
	svc, _, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	t.Setenv("ATTENDANCE_ASYNC", "true")
	if svc.AsyncEnabled() {
		t.Error("without a publisher the service must stay synchronous even with the flag on")
	}
	svc.SetEnqueuer(&fakeEnqueuer{})
	if !svc.AsyncEnabled() {
		t.Error("flag on + publisher = async")
	}
	t.Setenv("ATTENDANCE_ASYNC", "")
	if svc.AsyncEnabled() {
		t.Error("flag off = synchronous (the default)")
	}
}

func TestClockInAsyncPublishesWithServerTimeAndIgnoresClientStatus(t *testing.T) {
	svc, enq, repo := asyncService(officeAssignment(tod(0, 1), tod(23, 59)))
	claimed := "2020-01-01T08:00:00+07:00"
	req := atOffice()
	req.CheckIn, req.Status = &claimed, "present"
	acc, err := svc.ClockInAsync(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatal(err)
	}
	if acc.State != "pending" || acc.IdempotencyKey == "" || acc.StatusURL == "" {
		t.Errorf("accepted = %+v", acc)
	}
	if len(enq.events) != 1 || len(repo.inserted) != 0 {
		t.Fatalf("events=%d rows=%d: the API must publish and not write", len(enq.events), len(repo.inserted))
	}
	ev := enq.events[0]
	if ev.Status != "late" || time.Since(ev.ServerReceivedAt) > time.Minute {
		t.Errorf("status=%q received=%v: the server decides both", ev.Status, ev.ServerReceivedAt)
	}
	if ev.ClientTimestamp == nil || ev.ClientTimestamp.Year() != 2020 {
		t.Errorf("the device's claim must be kept for audit, got %v", ev.ClientTimestamp)
	}
}

func TestClockInAsyncPublishFailureIsAnErrorToTheClient(t *testing.T) {
	svc, enq, _ := asyncService(officeAssignment(tod(0, 1), tod(23, 59)))
	enq.err = errors.New("broker down")
	if _, err := svc.ClockInAsync(context.Background(), uuid.New(), atOffice()); !errors.Is(err, ErrQueueUnavailable) {
		t.Fatalf("err = %v, want ErrQueueUnavailable", err)
	}
}

func TestClockInAsyncStillChecksTheGeofence(t *testing.T) {
	svc, enq, _ := asyncService(officeAssignment(tod(0, 1), tod(23, 59)))
	far := &model.CreateAttendanceRequest{CheckInLatitude: "-6.3", CheckInLongitude: "106.8"}
	if _, err := svc.ClockInAsync(context.Background(), uuid.New(), far); !errors.Is(err, ErrOutsideGeofence) {
		t.Fatalf("err = %v", err)
	}
	if len(enq.events) != 0 {
		t.Error("an invalid request must not be queued")
	}
}

func TestClockOutAsyncUsesTheDerivedKeyAndToday(t *testing.T) {
	svc, enq, _ := asyncService(officeAssignment(tod(0, 0), tod(0, 1)))
	p := uuid.New()
	acc, err := svc.ClockOutAsync(context.Background(), p, &model.UpdateAttendanceRequest{CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"})
	if err != nil {
		t.Fatal(err)
	}
	ev := enq.events[0]
	day := attendanceDay(time.Now())
	if ev.Action != attendanceq.ActionCheckOut || ev.Day != day.Format("2006-01-02") || acc.IdempotencyKey != deriveIdempotencyKey(p, day, "checkout") {
		t.Errorf("event = %+v", ev)
	}
}

type statusRepo struct {
	repository.AttendanceRepository
	att     *model.Attendance
	action  string
	failure *model.AttendanceFailure
}

func (s statusRepo) FindByIdempotencyKey(context.Context, uuid.UUID, string) (*model.Attendance, string, error) {
	return s.att, s.action, nil
}
func (s statusRepo) FindFailure(context.Context, uuid.UUID, string) (*model.AttendanceFailure, error) {
	return s.failure, nil
}

func TestQueueStatusStates(t *testing.T) {
	id := uuid.New()
	cases := []struct {
		name string
		repo statusRepo
		want string
	}{
		{"nothing yet", statusRepo{}, "pending"},
		{"stored", statusRepo{att: &model.Attendance{ID: id}, action: "checkin"}, "recorded"},
		{"gave up", statusRepo{failure: &model.AttendanceFailure{Action: "checkout", Error: "boom"}}, "failed"},
	}
	for _, c := range cases {
		svc := &attendanceServiceImpl{repo: c.repo}
		st, err := svc.QueueStatus(context.Background(), uuid.New(), uuid.NewString())
		if err != nil || st.State != c.want {
			t.Errorf("%s: state=%v err=%v, want %s", c.name, st, err, c.want)
		}
	}
}
