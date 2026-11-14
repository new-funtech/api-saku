package attendanceq

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && v > 0 {
		return v
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil && v > 0 {
		return v
	}
	return def
}

// AsyncEnabled is the feature flag. Off (the default) keeps the synchronous behaviour exactly.
func AsyncEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ATTENDANCE_ASYNC"))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

// LoadConfig reads the queue settings.
//
//	ATTENDANCE_QUEUE               queue name (default attendance.events)
//	ATTENDANCE_RETRY_DELAYS        retry backoff ladder (default 2s,10s,60s,300s)
//	ATTENDANCE_MAX_ATTEMPTS        failures before a message goes to the DLQ (default 5)
//	ATTENDANCE_WORKER_PREFETCH     unacked messages the worker may hold (default 500)
//	ATTENDANCE_BATCH_SIZE          messages per DB batch (default 100)
//	ATTENDANCE_BATCH_WAIT          longest wait to fill a batch (default 1s)
//	ATTENDANCE_PUBLISH_POOL        confirm channels in the API (default 8)
//	ATTENDANCE_PUBLISH_TIMEOUT     how long the API waits for the broker's confirm (default 3s)
//	ATTENDANCE_WORKER_METRICS_ADDR worker metrics/health listener (default :9101)
//	ATTENDANCE_MAX_PHOTO_BYTES     largest photo accepted in async mode (default 2097152)
func LoadConfig() Config {
	c := Config{
		Queue:        strings.TrimSpace(os.Getenv("ATTENDANCE_QUEUE")),
		MaxAttempts:  envInt("ATTENDANCE_MAX_ATTEMPTS", 5),
		Prefetch:     envInt("ATTENDANCE_WORKER_PREFETCH", 500),
		BatchSize:    envInt("ATTENDANCE_BATCH_SIZE", 100),
		BatchWait:    envDur("ATTENDANCE_BATCH_WAIT", time.Second),
		PublishPool:  envInt("ATTENDANCE_PUBLISH_POOL", 8),
		PublishWait:  envDur("ATTENDANCE_PUBLISH_TIMEOUT", 3*time.Second),
		MetricsAddr:  strings.TrimSpace(os.Getenv("ATTENDANCE_WORKER_METRICS_ADDR")),
		MaxPhotoSize: envInt("ATTENDANCE_MAX_PHOTO_BYTES", 2*1024*1024),
	}
	if c.Queue == "" {
		c.Queue = "attendance.events"
	}
	if c.MetricsAddr == "" {
		c.MetricsAddr = ":9101"
	}
	c.RetryDelays = []time.Duration{2 * time.Second, 10 * time.Second, time.Minute, 5 * time.Minute}
	if raw := strings.TrimSpace(os.Getenv("ATTENDANCE_RETRY_DELAYS")); raw != "" {
		var ds []time.Duration
		for _, p := range strings.Split(raw, ",") {
			if d, err := time.ParseDuration(strings.TrimSpace(p)); err == nil && d > 0 {
				ds = append(ds, d)
			}
		}
		if len(ds) > 0 {
			c.RetryDelays = ds
		}
	}
	return c
}
