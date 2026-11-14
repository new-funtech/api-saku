// attendance-worker stores the clock-in/out events the API publishes when ATTENDANCE_ASYNC=true.
//
//	attendance-worker                      run until SIGINT/SIGTERM (finishes the batch in hand first)
//	attendance-worker -reprocess-dlq       move the dead-letter queue back to the main queue, then exit
//	attendance-worker -reprocess-dlq -limit 100
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ganiramadhan/ganipedia/backend/internal/attendanceq"
	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
)

func main() {
	reprocess := flag.Bool("reprocess-dlq", false, "move dead-lettered messages back to the main queue and exit")
	limit := flag.Int("limit", 0, "with -reprocess-dlq: at most this many messages (0 = all)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config.LoadEnv()
	config.ConnectDatabase()
	config.RunMigrations()
	config.ConnectS3()
	config.ConnectRabbitMQ()
	defer config.CloseConnections()

	if config.RabbitClient == nil {
		log.Fatal("attendance-worker: RabbitMQ is not configured (RABBITMQ_HOST / RABBITMQ_USER)")
	}
	cfg := attendanceq.LoadConfig()
	worker, err := attendanceq.NewWorker(config.DB, config.RabbitClient, cfg, utils.UploadBase64DataURL, logger)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *reprocess {
		n, err := worker.ReprocessDLQ(ctx, *limit)
		logger.Info("dlq reprocessed", "moved", n, "error", err)
		if err != nil {
			os.Exit(1)
		}
		return
	}

	go worker.ServeMetrics(ctx)
	logger.Info("attendance worker started", "queue", cfg.Queue, "batch", cfg.BatchSize, "wait", cfg.BatchWait.String(),
		"prefetch", cfg.Prefetch, "max_attempts", cfg.MaxAttempts, "metrics", cfg.MetricsAddr)
	if err := worker.Run(ctx); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("attendance worker stopped")
}
