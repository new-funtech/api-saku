package attendanceq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/pkg/broker/rabbitmq"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UploadFunc stores a base64 data URL and returns the object key (utils.UploadBase64DataURL).
type UploadFunc func(ctx context.Context, dataURL, folder string) (string, error)

// Worker stores attendance events. Delivery is at-least-once, so everything it does is idempotent:
// check-ins are INSERT ... ON CONFLICT DO NOTHING, check-outs are UPDATE ... WHERE check_out IS NULL.
type Worker struct {
	db     *gorm.DB
	client *rabbitmq.Client
	pub    *rabbitmq.Publisher
	cfg    Config
	upload UploadFunc
	log    *slog.Logger
	M      *Metrics
}

func NewWorker(db *gorm.DB, client *rabbitmq.Client, cfg Config, upload UploadFunc, log *slog.Logger) (*Worker, error) {
	if err := client.Declare(Topology(cfg)); err != nil {
		return nil, fmt.Errorf("attendance worker: declare queues: %w", err)
	}
	return &Worker{db: db, client: client, pub: client.NewPublisher(4), cfg: cfg, upload: upload, log: log, M: &Metrics{}}, nil
}

// Run consumes until ctx is cancelled, reconnecting after broker errors. On cancel the batch in
// hand is finished and acked before it returns.
func (w *Worker) Run(ctx context.Context) error {
	go w.pollDepths(ctx)
	for ctx.Err() == nil {
		err := w.client.ConsumeBatches(ctx, w.cfg.Queue, w.cfg.Prefetch, w.cfg.BatchSize, w.cfg.BatchWait, w.handleBatch)
		if ctx.Err() != nil {
			return nil
		}
		w.log.Error("consumer stopped, reconnecting", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
	return nil
}

// ServeMetrics exposes /metrics (Prometheus text) and /health until ctx is cancelled.
func (w *Worker) ServeMetrics(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.M.WritePrometheus(rw)
	})
	mux.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) { _, _ = rw.Write([]byte("ok")) })
	srv := &http.Server{Addr: w.cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		w.log.Error("metrics server", "error", err)
	}
}

func (w *Worker) pollDepths(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	t := Topology(w.cfg)
	for {
		if n, err := w.client.QueueDepth(t.Queue); err == nil {
			w.M.QueueDepth.Store(int64(n))
		}
		if n, err := w.client.QueueDepth(t.DLQ()); err == nil {
			w.M.DLQDepth.Store(int64(n))
		}
		var retry int
		for i := range t.RetryDelays {
			if n, err := w.client.QueueDepth(t.RetryQueue(i)); err == nil {
				retry += n
			}
		}
		w.M.RetryDepth.Store(int64(retry))
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

type item struct {
	d       amqp.Delivery
	ev      Event
	attempt int
	photo   *string // uploaded object key
	doc     *string
	err     error // set when the item must be retried or failed
	perm    bool  // err is permanent: record it, do not retry
	replay  bool
}

func attemptOf(d amqp.Delivery) int {
	switch v := d.Headers[rabbitmq.HeaderAttempt].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func (w *Worker) handleBatch(ctx context.Context, batch []amqp.Delivery) {
	started := time.Now()
	w.M.Batches.Add(1)
	w.M.BatchMessages.Add(int64(len(batch)))
	w.M.Received.Add(int64(len(batch)))

	var checkIns, checkOuts []*item
	var oldest time.Time
	for _, d := range batch {
		it := &item{d: d, attempt: attemptOf(d)}
		if err := json.Unmarshal(d.Body, &it.ev); err != nil || it.ev.PersonnelID == uuid.Nil || it.ev.Key == "" {
			w.deadLetter(ctx, it, fmt.Errorf("invalid message: %v", err))
			continue
		}
		if oldest.IsZero() || it.ev.ServerReceivedAt.Before(oldest) {
			oldest = it.ev.ServerReceivedAt
		}
		switch it.ev.Action {
		case ActionCheckIn:
			checkIns = append(checkIns, it)
		case ActionCheckOut:
			checkOuts = append(checkOuts, it)
		default:
			w.deadLetter(ctx, it, fmt.Errorf("unknown action %q", it.ev.Action))
		}
	}

	w.uploadMedia(ctx, checkIns)
	w.uploadMedia(ctx, checkOuts)
	w.storeCheckIns(ctx, checkIns)
	for _, it := range checkOuts {
		if it.err == nil {
			w.storeCheckOut(ctx, it)
		}
	}
	for _, it := range append(checkIns, checkOuts...) {
		w.settle(ctx, it)
	}

	took := time.Since(started)
	w.M.LastBatchMs.Store(took.Milliseconds())
	if !oldest.IsZero() {
		lag := time.Since(oldest).Milliseconds()
		w.M.OldestLagMs.Store(lag)
		if lag > w.M.MaxOldestLagMs.Load() {
			w.M.MaxOldestLagMs.Store(lag)
		}
	}
	w.log.Info("batch", "messages", len(batch), "checkins", len(checkIns), "checkouts", len(checkOuts), "took_ms", took.Milliseconds(),
		"queue_wait_ms", w.M.OldestLagMs.Load())
}

// uploadMedia moves the photo/document out of the message into object storage.
func (w *Worker) uploadMedia(ctx context.Context, items []*item) {
	for _, it := range items {
		folder := "ATTENDANCES/" + it.ev.PersonnelID.String()
		if it.ev.Photo != "" && w.upload != nil {
			key, err := w.upload(ctx, it.ev.Photo, folder)
			if err != nil {
				it.err = fmt.Errorf("upload photo: %w", err)
				continue
			}
			it.photo = &key
		}
		if it.ev.Document != "" && w.upload != nil {
			key, err := w.upload(ctx, it.ev.Document, folder)
			if err != nil {
				it.err = fmt.Errorf("upload document: %w", err)
				continue
			}
			it.doc = &key
		}
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (it *item) row() (*model.Attendance, error) {
	day, err := time.Parse("2006-01-02", it.ev.Day)
	if err != nil {
		return nil, fmt.Errorf("invalid day %q", it.ev.Day)
	}
	received := it.ev.ServerReceivedAt
	key := it.ev.Key
	return &model.Attendance{
		ID: uuid.New(), PersonnelID: it.ev.PersonnelID, LocationID: it.ev.LocationID, AssignmentID: it.ev.AssignmentID,
		Date: day, CheckIn: &received, CheckInLatitude: strPtr(it.ev.Latitude), CheckInLongitude: strPtr(it.ev.Longitude),
		CheckInPhoto: it.photo, SupportingDocument: it.doc, Status: it.ev.Status, Notes: it.ev.Notes,
		IdempotencyKey: &key, ServerReceivedAt: &received, ClientTimestamp: it.ev.ClientTimestamp,
	}, nil
}

// storeCheckIns writes all check-ins of the batch in one multi-row INSERT. If that fails (one bad
// row, e.g. a deleted office, would fail them all) it falls back to one INSERT per message so only
// the bad one is retried.
func (w *Worker) storeCheckIns(ctx context.Context, items []*item) {
	var pending []*item
	var rows []*model.Attendance
	for _, it := range items {
		if it.err != nil {
			continue
		}
		row, err := it.row()
		if err != nil {
			it.err, it.perm = err, true
			continue
		}
		pending, rows = append(pending, it), append(rows, row)
	}
	if len(rows) == 0 {
		return
	}
	tx := w.db.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).Clauses(clause.OnConflict{DoNothing: true})
	res := tx.CreateInBatches(rows, len(rows))
	if res.Error == nil {
		w.M.Recorded.Add(res.RowsAffected)
		w.M.Replays.Add(int64(len(rows)) - res.RowsAffected)
		return
	}
	w.log.Warn("batch insert failed, retrying one by one", "error", res.Error, "rows", len(rows))
	for _, it := range pending {
		row, _ := it.row()
		one := w.db.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		switch {
		case one.Error != nil:
			it.err = one.Error
		case one.RowsAffected == 0:
			w.M.Replays.Add(1)
		default:
			w.M.Recorded.Add(1)
		}
	}
}

const checkOutSQL = `UPDATE attendances SET
	check_out = @at,
	check_out_latitude = @lat,
	check_out_longitude = @lng,
	check_out_photo = COALESCE(@photo, check_out_photo),
	supporting_document = COALESCE(@doc, supporting_document),
	work_duration = GREATEST(0, (EXTRACT(EPOCH FROM (CAST(@at AS timestamptz) - check_in)) / 60)::int),
	check_out_idempotency_key = @key,
	updated_at = now()
WHERE personnel_id = @p AND date = @day AND check_out IS NULL`

func (w *Worker) storeCheckOut(ctx context.Context, it *item) {
	day, err := time.Parse("2006-01-02", it.ev.Day)
	if err != nil {
		it.err, it.perm = fmt.Errorf("invalid day %q", it.ev.Day), true
		return
	}
	res := w.db.WithContext(ctx).Exec(checkOutSQL, map[string]interface{}{
		"at": it.ev.ServerReceivedAt, "lat": strPtr(it.ev.Latitude), "lng": strPtr(it.ev.Longitude),
		"photo": it.photo, "doc": it.doc, "key": it.ev.Key, "p": it.ev.PersonnelID, "day": day,
	})
	if res.Error != nil {
		it.err = res.Error
		return
	}
	if res.RowsAffected > 0 {
		w.M.Recorded.Add(1)
		return
	}
	// Nothing updated: it is already closed (a replay or a conflict), or the check-in has not been stored yet.
	var att model.Attendance
	err = w.db.WithContext(ctx).Select("id", "check_out", "check_out_idempotency_key").
		Where("personnel_id = ? AND date = ?", it.ev.PersonnelID, day).First(&att).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		it.err = errors.New("no attendance for the day yet (check-in not stored); will retry")
	case err != nil:
		it.err = err
	case att.CheckOutIdempotencyKey != nil && *att.CheckOutIdempotencyKey == it.ev.Key:
		it.replay = true
		w.M.Replays.Add(1)
	default:
		it.err, it.perm = errors.New("attendance already checked out with another request"), true
	}
}

// settle acks, retries or dead-letters one message according to its outcome.
func (w *Worker) settle(ctx context.Context, it *item) {
	switch {
	case it.err == nil:
		_ = it.d.Ack(false)
	case it.perm:
		w.recordFailure(ctx, it, it.err)
		w.M.Failed.Add(1)
		_ = it.d.Ack(false)
	default:
		next := it.attempt + 1
		if next >= w.cfg.MaxAttempts {
			w.deadLetter(ctx, it, it.err)
			return
		}
		t := Topology(w.cfg)
		idx := it.attempt
		if idx >= len(t.RetryDelays) {
			idx = len(t.RetryDelays) - 1
		}
		hdr := amqp.Table{rabbitmq.HeaderAttempt: int32(next), rabbitmq.HeaderError: truncate(it.err.Error(), 500)}
		pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := w.pub.Publish(pctx, t.RetryQueue(idx), it.d.Body, hdr); err != nil {
			w.log.Error("retry publish failed; message stays in the queue", "error", err)
			_ = it.d.Nack(false, true)
			return
		}
		w.M.Retried.Add(1)
		w.log.Warn("will retry", "key", it.ev.Key, "attempt", next, "error", it.err.Error())
		_ = it.d.Ack(false)
	}
}

// deadLetter parks the message in the DLQ (with the reason) and records the failure for the status endpoint.
func (w *Worker) deadLetter(ctx context.Context, it *item, cause error) {
	t := Topology(w.cfg)
	hdr := amqp.Table{rabbitmq.HeaderAttempt: int32(it.attempt + 1), rabbitmq.HeaderError: truncate(cause.Error(), 500)}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.pub.Publish(pctx, t.DLQ(), it.d.Body, hdr); err != nil {
		w.log.Error("DLQ publish failed; message stays in the queue", "error", err)
		_ = it.d.Nack(false, true)
		return
	}
	w.recordFailure(ctx, it, cause)
	w.M.DeadLettered.Add(1)
	w.log.Error("dead-lettered", "key", it.ev.Key, "action", it.ev.Action, "error", cause.Error())
	_ = it.d.Ack(false)
}

func (w *Worker) recordFailure(ctx context.Context, it *item, cause error) {
	if it.ev.PersonnelID == uuid.Nil || it.ev.Key == "" {
		return
	}
	f := model.AttendanceFailure{
		ID: uuid.New(), PersonnelID: it.ev.PersonnelID, IdempotencyKey: it.ev.Key, Action: string(it.ev.Action),
		Error: truncate(cause.Error(), 1000), Attempts: it.attempt + 1,
	}
	if err := w.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&f).Error; err != nil {
		w.log.Error("record failure", "error", err)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ReprocessDLQ moves messages from the dead-letter queue back to the main queue (attempt counter
// reset) and clears their failure rows, so they are tried again, e.g. after the cause was fixed.
func (w *Worker) ReprocessDLQ(ctx context.Context, limit int) (int, error) {
	t := Topology(w.cfg)
	n, err := w.client.Requeue(ctx, t.DLQ(), t.Queue, limit, w.pub)
	if n > 0 {
		// The failure rows are only informative; drop them so the status goes back to "pending".
		_ = w.db.WithContext(ctx).Exec("DELETE FROM attendance_failures").Error
	}
	return n, err
}
