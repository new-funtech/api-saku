package attendanceq

import (
	"fmt"
	"io"
	"sync/atomic"
)

// Metrics are plain counters and gauges, exposed in the Prometheus text format by the worker.
type Metrics struct {
	Received       atomic.Int64 // messages taken from the queue
	Recorded       atomic.Int64 // rows written (check-in) or closed (check-out)
	Replays        atomic.Int64 // already stored: the duplicate was recognised and ignored
	Retried        atomic.Int64 // sent to a retry queue
	DeadLettered   atomic.Int64 // parked in the dead-letter queue
	Failed         atomic.Int64 // permanent failures recorded (e.g. checkout on a closed attendance)
	Batches        atomic.Int64
	BatchMessages  atomic.Int64
	QueueDepth     atomic.Int64
	RetryDepth     atomic.Int64
	DLQDepth       atomic.Int64
	LastBatchMs    atomic.Int64
	OldestLagMs    atomic.Int64 // age of the oldest message in the last batch (queue wait)
	MaxOldestLagMs atomic.Int64
}

func (m *Metrics) WritePrometheus(w io.Writer) {
	line := func(name, help, typ string, v int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %d\n", name, help, name, typ, name, v)
	}
	line("attendance_queue_depth", "Messages waiting in the attendance queue.", "gauge", m.QueueDepth.Load())
	line("attendance_retry_depth", "Messages waiting in the retry queues.", "gauge", m.RetryDepth.Load())
	line("attendance_dlq_depth", "Messages in the dead-letter queue.", "gauge", m.DLQDepth.Load())
	line("attendance_received_total", "Messages taken from the queue.", "counter", m.Received.Load())
	line("attendance_recorded_total", "Attendance rows written or closed.", "counter", m.Recorded.Load())
	line("attendance_replays_total", "Messages recognised as already stored.", "counter", m.Replays.Load())
	line("attendance_retried_total", "Messages sent to a retry queue.", "counter", m.Retried.Load())
	line("attendance_dead_lettered_total", "Messages parked in the dead-letter queue.", "counter", m.DeadLettered.Load())
	line("attendance_failed_total", "Permanent failures recorded.", "counter", m.Failed.Load())
	line("attendance_batches_total", "Batches processed.", "counter", m.Batches.Load())
	line("attendance_batch_messages_total", "Messages processed in batches.", "counter", m.BatchMessages.Load())
	line("attendance_last_batch_ms", "Duration of the last batch.", "gauge", m.LastBatchMs.Load())
	line("attendance_queue_wait_ms", "Queue wait of the oldest message in the last batch.", "gauge", m.OldestLagMs.Load())
	line("attendance_queue_wait_max_ms", "Largest queue wait seen since start.", "gauge", m.MaxOldestLagMs.Load())
}
