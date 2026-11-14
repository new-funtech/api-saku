package services

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/google/uuid"
)

func TestClockInSameKeyTwiceIsOneRow(t *testing.T) {
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	p := uuid.New()
	req := func() *model.CreateAttendanceRequest {
		r := atOffice()
		r.IdempotencyKey = "11111111-1111-4111-8111-111111111111"
		return r
	}
	first, replayed, err := svc.ClockIn(context.Background(), p, req())
	if err != nil || replayed {
		t.Fatalf("first: replayed=%v err=%v", replayed, err)
	}
	second, replayed, err := svc.ClockIn(context.Background(), p, req())
	if err != nil {
		t.Fatalf("a duplicate must be answered as a success, got %v", err)
	}
	if !replayed || second.ID != first.ID {
		t.Errorf("replayed=%v ids %v/%v: the duplicate must return the first record", replayed, first.ID, second.ID)
	}
	if len(repo.inserted) != 1 {
		t.Errorf("rows = %d, want 1", len(repo.inserted))
	}
}

func TestClockInDifferentKeySameDayStillOneRow(t *testing.T) {
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	p := uuid.New()
	a, b := atOffice(), atOffice()
	a.IdempotencyKey, b.IdempotencyKey = uuid.NewString(), uuid.NewString()
	first, _, _ := svc.ClockIn(context.Background(), p, a)
	second, replayed, err := svc.ClockIn(context.Background(), p, b)
	if err != nil || !replayed || second.ID != first.ID || len(repo.inserted) != 1 {
		t.Fatalf("err=%v replayed=%v rows=%d: one employee, one row per day", err, replayed, len(repo.inserted))
	}
}

func TestClockInWithoutKeyUsesADerivedKey(t *testing.T) {
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	p := uuid.New()
	if _, _, err := svc.ClockIn(context.Background(), p, atOffice()); err != nil {
		t.Fatal(err)
	}
	got := repo.inserted[0].IdempotencyKey
	want := deriveIdempotencyKey(p, attendanceDay(*repo.inserted[0].CheckIn), "checkin")
	if got == nil || *got != want {
		t.Errorf("key = %v, want the derived %s", got, want)
	}
	if deriveIdempotencyKey(p, repo.inserted[0].Date, "checkin") == deriveIdempotencyKey(p, repo.inserted[0].Date, "checkout") {
		t.Error("check-in and check-out must not share a derived key")
	}
}

func TestSimultaneousIdenticalClockInsMakeOneRow(t *testing.T) {
	svc, repo, _ := newClockService(officeAssignment(tod(0, 1), tod(23, 59)))
	p := uuid.New()
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	replays := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, replayed, err := svc.ClockIn(context.Background(), p, atOffice())
			errs <- err
			replays <- replayed
		}()
	}
	wg.Wait()
	close(errs)
	close(replays)
	for err := range errs {
		if err != nil {
			t.Fatalf("every duplicate must succeed, got %v", err)
		}
	}
	newOnes := 0
	for r := range replays {
		if !r {
			newOnes++
		}
	}
	if len(repo.inserted) != 1 || newOnes != 1 {
		t.Errorf("rows=%d new=%d, want exactly one of each", len(repo.inserted), newOnes)
	}
}

func TestClockOutRetryWithTheSameKeyIsASuccessAndKeepsTheFirstTime(t *testing.T) {
	a := officeAssignment(tod(0, 0), tod(0, 1))
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row
	key := uuid.NewString()
	req := func() *model.UpdateAttendanceRequest {
		return &model.UpdateAttendanceRequest{IdempotencyKey: key, CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"}
	}
	if _, replayed, err := svc.ClockOut(context.Background(), p, req()); err != nil || replayed {
		t.Fatalf("first: replayed=%v err=%v", replayed, err)
	}
	first := *repo.rows[row.ID].CheckOut
	resp, replayed, err := svc.ClockOut(context.Background(), p, req())
	if err != nil || !replayed {
		t.Fatalf("same key: replayed=%v err=%v, want a replayed success", replayed, err)
	}
	if resp.CheckOut == nil || !repo.rows[row.ID].CheckOut.Equal(first) || repo.completed != 1 {
		t.Error("the replay must not change the stored check-out")
	}
}

func TestClockOutWithoutKeyTwiceIsAReplay(t *testing.T) {
	a := officeAssignment(tod(0, 0), tod(0, 1))
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row
	req := &model.UpdateAttendanceRequest{CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"}
	if _, _, err := svc.ClockOut(context.Background(), p, req); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := svc.ClockOut(context.Background(), p, req); err != nil || !replayed {
		t.Fatalf("derived key: replayed=%v err=%v", replayed, err)
	}
}

func TestSimultaneousClockOutsWriteOnce(t *testing.T) {
	a := officeAssignment(tod(0, 0), tod(0, 1))
	svc, repo, _ := newClockService(a)
	p := uuid.New()
	row := openRow(p, a)
	repo.rows[row.ID] = row
	const n = 40
	var wg sync.WaitGroup
	conflicts := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := svc.ClockOut(context.Background(), p, &model.UpdateAttendanceRequest{CheckOutLatitude: "-6.2", CheckOutLongitude: "106.816666"})
			conflicts <- errors.Is(err, ErrAlreadyCheckedOut)
		}()
	}
	wg.Wait()
	close(conflicts)
	for c := range conflicts {
		if c {
			t.Error("same derived key: a simultaneous repeat must be a replayed success, not a conflict")
		}
	}
	if repo.completed != 1 {
		t.Errorf("writes = %d, want exactly 1", repo.completed)
	}
}
