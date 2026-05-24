package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubilayciftci/insider-one-notification/internal/adapters/kafka"
	pgadapter "github.com/kubilayciftci/insider-one-notification/internal/adapters/postgres"
	"github.com/kubilayciftci/insider-one-notification/internal/adapters/webhook"
	"github.com/kubilayciftci/insider-one-notification/internal/config"
	"github.com/kubilayciftci/insider-one-notification/internal/telemetry"
	"github.com/kubilayciftci/insider-one-notification/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()
	logger := telemetry.NewLogger(cfg.ServiceName)

	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.ServiceName+"-worker", cfg.JaegerEndpoint)
	if err != nil {
		logger.Error("init tracer failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer shutdownTracer(ctx) //nolint:errcheck

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to database failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	repo := pgadapter.NewRepository(pool)
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close() //nolint:errcheck
	notifier := webhook.NewClient(cfg.WebhookURL, 10)
	metrics := telemetry.NewMetrics()

	w := worker.New(
		repo,
		notifier,
		producer,
		logger,
		metrics,
		cfg.KafkaBrokers,
		cfg.RateLimitPerSec,
		cfg.MaxRetries,
		cfg.BaseRetryDelay,
	)

	go w.StartScheduler(ctx, 10*time.Second)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		logger.Info("worker metrics server starting", slog.String("port", ":9090"))
		if err := http.ListenAndServe(":9090", mux); err != nil {
			logger.Error("metrics server error", slog.Any("error", err))
		}
	}()

	go func() {
		logger.Info("worker starting")
		if err := w.Start(ctx); err != nil {
			logger.Error("worker error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker...")
	cancel()
	logger.Info("worker stopped")
}
