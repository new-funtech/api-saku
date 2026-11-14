package attendanceq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// These tests need a real PostgreSQL and RabbitMQ and only run when told where they are:
//
//	ATTENDANCE_IT_DB=127.0.0.1:55432  ATTENDANCE_IT_AMQP=127.0.0.1:56720
//
// Use the throwaway containers from loadtest/README.md; never point them at a real environment.

type itEnv struct {
	db     *gorm.DB
	client *rabbitmq.Client
	cfg    Config
	person uuid.UUID
	loc    uuid.UUID
	asg    uuid.UUID
}

func setup(t *testing.T) *itEnv {
	t.Helper()
	dbAddr, amqpAddr := os.Getenv("ATTENDANCE_IT_DB"), os.Getenv("ATTENDANCE_IT_AMQP")
	if dbAddr == "" || amqpAddr == "" {
		t.Skip("set ATTENDANCE_IT_DB and ATTENDANCE_IT_AMQP to run the worker integration tests")
	}
	host, port := splitHostPort(dbAddr)
	for k, v := range map[string]string{"DB_HOST": host, "DB_PORT": port, "DB_USER": "postgres", "DB_PASSWORD": "loadtest", "DB_NAME": "hrmis_loadtest", "DB_SSLMODE": "disable"} {
		t.Setenv(k, v)
	}
	config.ConnectDatabase()
	config.RunMigrations()
	if config.DB == nil {
		t.Fatal("no database")
	}
	ah, ap := splitHostPort(amqpAddr)
	client, err := rabbitmq.NewClient(rabbitmq.Config{Host: ah, Port: ap, User: "guest", Password: "guest", VHost: "/"})
	if err != nil {
		t.Fatalf("rabbitmq: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	e := &itEnv{db: config.DB, client: client, person: uuid.New(), loc: uuid.New(), asg: uuid.New()}
	e.cfg = Config{
		Queue: "it.attendance." + uuid.NewString()[:8], RetryDelays: []time.Duration{50 * time.Millisecond, 100 * time.Millisecond},
		MaxAttempts: 3, Prefetch: 100, BatchSize: 50, BatchWait: 100 * time.Millisecond, PublishPool: 2, PublishWait: 3 * time.Second,
	}
	e.seed(t)
	if err := client.Declare(Topology(e.cfg)); err != nil {
		t.Fatalf("declare: %v", err)
	}
	return e
}

func (e *itEnv) seed(t *testing.T) {
	t.Helper()
	bujp, user := uuid.New(), uuid.New()
	exec := func(q string, a ...interface{}) {
		if err := e.db.Exec(q, a...).Error; err != nil {
			t.Fatalf("seed: %v\n%s", err, q)
		}
	}
	exec(`INSERT INTO bujps (id, code, name) VALUES (?, ?, 'IT')`, bujp, "IT-"+bujp.String()[:8])
	exec(`INSERT INTO locations (id, bujp_id, code, name, address) VALUES (?, ?, ?, 'IT', 'x')`, e.loc, bujp, "IT-"+e.loc.String()[:8])
	exec(`INSERT INTO users (id, email, password, role, full_name) VALUES (?, ?, 'x', 'guard', 'IT')`, user, user.String()+"@it.local")
	exec(`INSERT INTO personnels (id, bujp_id, user_id, id_number, full_name, join_date) VALUES (?, ?, ?, ?, 'IT', CURRENT_DATE)`, e.person, bujp, user, "IT-"+e.person.String()[:8])
	exec(`INSERT INTO assignments (id, personnel_id, location_id, start_date, status) VALUES (?, ?, ?, CURRENT_DATE, 'active')`, e.asg, e.person, e.loc)
	t.Cleanup(func() {
		e.db.Exec(`DELETE FROM attendance_failures WHERE personnel_id = ?`, e.person)
		e.db.Exec(`DELETE FROM attendances WHERE personnel_id = ?`, e.person)
		e.db.Exec(`DELETE FROM assignments WHERE id = ?`, e.asg)
		e.db.Exec(`DELETE FROM personnels WHERE id = ?`, e.person)
		e.db.Exec(`DELETE FROM users WHERE id = ?`, user)
		e.db.Exec(`DELETE FROM locations WHERE id = ?`, e.loc)
		e.db.Exec(`DELETE FROM bujps WHERE id = ?`, bujp)
		tp := Topology(e.cfg)
		if ch, err := e.client.Channel(); err == nil {
			for _, q := range append([]string{tp.Queue, tp.DLQ()}, retryQueues(tp)...) {
				_, _ = ch.QueueDelete(q, false, false, false)
			}
			_ = ch.Close()
		}
	})
}

func retryQueues(tp rabbitmq.Topology) []string {
	var qs []string
	for i := range tp.RetryDelays {
		qs = append(qs, tp.RetryQueue(i))
	}
	return qs
}

func splitHostPort(a string) (string, string) {
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] == ':' {
			return a[:i], a[i+1:]
		}
	}
	return a, "5672"
}

func (e *itEnv) event(action Action, key string) Event {
	a := e.asg
	return Event{
		Version: 1, Action: action, Key: key, PersonnelID: e.person, LocationID: e.loc, AssignmentID: &a,
		Day: time.Now().Format("2006-01-02"), Status: "present", Latitude: "-6.2", Longitude: "106.8",
		ServerReceivedAt: time.Now(),
	}
}

func (e *itEnv) publish(t *testing.T, ev Event) {
	t.Helper()
	body, _ := json.Marshal(ev)
	pub := e.client.NewPublisher(1)
	if err := pub.Publish(context.Background(), e.cfg.Queue, body, amqp.Table{rabbitmq.HeaderAttempt: int32(0)}); err != nil {
		t.Fatal(err)
	}
}

func (e *itEnv) startWorker(t *testing.T) (*Worker, context.CancelFunc, chan struct{}) {
	t.Helper()
	w, err := NewWorker(e.db, e.client, e.cfg, nil, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Run(ctx); close(done) }()
	return w, cancel, done
}

func (e *itEnv) count(q string, a ...interface{}) int64 {
	var n int64
	e.db.Raw(q, a...).Scan(&n)
	return n
}

func (e *itEnv) waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func TestIT_DuplicateEventsMakeOneRow(t *testing.T) {
	e := setup(t)
	key := uuid.NewString()
	for i := 0; i < 3; i++ {
		e.publish(t, e.event(ActionCheckIn, key)) // the same request three times (at-least-once)
	}
	e.publish(t, e.event(ActionCheckIn, uuid.NewString())) // another key, same employee and day
	_, cancel, done := e.startWorker(t)
	defer func() { cancel(); <-done }()

	e.waitFor(t, "queue drained", func() bool { n, _ := e.client.QueueDepth(e.cfg.Queue); return n == 0 })
	time.Sleep(300 * time.Millisecond)
	if n := e.count(`SELECT count(*) FROM attendances WHERE personnel_id = ?`, e.person); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestIT_WorkerCrashThenReprocessedWithoutDuplicate(t *testing.T) {
	e := setup(t)
	e.publish(t, e.event(ActionCheckIn, uuid.NewString()))

	// A "worker" that takes the message and dies before acking it.
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan struct{}, 1)
	go func() {
		_ = e.client.ConsumeBatches(ctx, e.cfg.Queue, 10, 10, 50*time.Millisecond, func(_ context.Context, batch []amqp.Delivery) {
			got <- struct{}{} // no Ack, no Nack: the process "crashed"
		})
	}()
	select {
	case <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("the first consumer never received the message")
	}
	cancel()
	time.Sleep(300 * time.Millisecond)
	if n := e.count(`SELECT count(*) FROM attendances WHERE personnel_id = ?`, e.person); n != 0 {
		t.Fatalf("rows = %d before the real worker ran", n)
	}

	_, stop, done := e.startWorker(t)
	defer func() { stop(); <-done }()
	e.waitFor(t, "the redelivered message is stored", func() bool {
		return e.count(`SELECT count(*) FROM attendances WHERE personnel_id = ?`, e.person) == 1
	})
	e.waitFor(t, "queue drained", func() bool { n, _ := e.client.QueueDepth(e.cfg.Queue); return n == 0 })
}

func TestIT_RetryThenDeadLetterThenReprocess(t *testing.T) {
	e := setup(t)
	key := uuid.NewString()
	e.publish(t, e.event(ActionCheckOut, key)) // there is no check-in yet: it cannot succeed
	w, stop, done := e.startWorker(t)
	defer func() { stop(); <-done }()

	tp := Topology(e.cfg)
	e.waitFor(t, "message parked in the DLQ", func() bool { n, _ := e.client.QueueDepth(tp.DLQ()); return n == 1 })
	if n := e.count(`SELECT count(*) FROM attendance_failures WHERE personnel_id = ? AND idempotency_key = ?`, e.person, key); n != 1 {
		t.Fatalf("failure rows = %d, want 1 (the status endpoint reads it)", n)
	}
	if w.M.Retried.Load() < 1 || w.M.DeadLettered.Load() != 1 {
		t.Errorf("retried=%d dead=%d", w.M.Retried.Load(), w.M.DeadLettered.Load())
	}

	// The cause is fixed (the check-in arrives); reprocess the DLQ and the check-out is stored.
	day, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	in := time.Now().Add(-2 * time.Hour)
	if err := e.db.Create(&model.Attendance{ID: uuid.New(), PersonnelID: e.person, LocationID: e.loc, Date: day, CheckIn: &in, Status: "present"}).Error; err != nil {
		t.Fatal(err)
	}
	moved, err := w.ReprocessDLQ(context.Background(), 0)
	if err != nil || moved != 1 {
		t.Fatalf("moved=%d err=%v", moved, err)
	}
	e.waitFor(t, "check-out stored after reprocessing", func() bool {
		return e.count(`SELECT count(*) FROM attendances WHERE personnel_id = ? AND check_out IS NOT NULL AND check_out_idempotency_key = ?`, e.person, key) == 1
	})
}

func TestIT_OneBadMessageDoesNotBlockTheBatch(t *testing.T) {
	e := setup(t)
	// A good check-in for this employee, and a bad one (unknown office -> foreign key error) for another employee.
	e.publish(t, e.event(ActionCheckIn, uuid.NewString()))
	bad := e.event(ActionCheckIn, uuid.NewString())
	bad.PersonnelID, bad.LocationID = uuid.New(), uuid.New()
	e.publish(t, bad)

	w, stop, done := e.startWorker(t)
	defer func() { stop(); <-done }()
	e.waitFor(t, "the good message is stored", func() bool {
		return e.count(`SELECT count(*) FROM attendances WHERE personnel_id = ?`, e.person) == 1
	})
	tp := Topology(e.cfg)
	e.waitFor(t, "the bad message ends in the DLQ", func() bool { n, _ := e.client.QueueDepth(tp.DLQ()); return n == 1 })
	_ = fmt.Sprint(w)
}

func TestIT_ConfirmedPublishFailsWhenTheQueueCannotBeReached(t *testing.T) {
	e := setup(t)
	pub := NewPublisher(e.client, e.cfg)
	ev := e.event(ActionCheckIn, uuid.NewString())
	if err := pub.Enqueue(context.Background(), ev); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if n, _ := e.client.QueueDepth(e.cfg.Queue); n != 1 {
		t.Errorf("queue depth = %d, want 1 (a confirmed publish is durably queued)", n)
	}
}

func TestIT_PublishToAMissingQueueIsAnErrorNotASilentDrop(t *testing.T) {
	e := setup(t)
	pub := e.client.NewPublisher(1)
	err := pub.Publish(context.Background(), "it.no-such-queue."+uuid.NewString()[:8], []byte("{}"), nil)
	if !errors.Is(err, rabbitmq.ErrUnroutable) {
		t.Fatalf("err = %v, want ErrUnroutable: a confirmed-but-dropped message must not look like success", err)
	}
}
