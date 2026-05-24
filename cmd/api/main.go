package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubilayciftci/insider-one-notification/internal/adapters/kafka"
	pgadapter "github.com/kubilayciftci/insider-one-notification/internal/adapters/postgres"
	"github.com/kubilayciftci/insider-one-notification/internal/adapters/rest"
	"github.com/kubilayciftci/insider-one-notification/internal/config"
	"github.com/kubilayciftci/insider-one-notification/internal/core/service"
	"github.com/kubilayciftci/insider-one-notification/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()
	logger := telemetry.NewLogger(cfg.ServiceName)

	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.ServiceName, cfg.JaegerEndpoint)
	if err != nil {
		logger.Error("init tracer failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer shutdownTracer(ctx)

	runMigrations(cfg.DatabaseURL, logger)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to database failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	repo := pgadapter.NewRepository(pool)
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	svc := service.NewNotificationService(repo, producer, logger)
	wsHub := rest.NewWSHub(logger)
	go wsHub.ListenPostgres(ctx, pool)
	handler := rest.NewHandler(svc, logger, wsHub)

	router := chi.NewRouter()
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.RequestID)
	router.Use(rest.TracingMiddleware)
	router.Use(rest.LoggingMiddleware(logger))
	router.Mount("/api/v1", handler.Routes())
	router.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         cfg.APIPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("API server starting", slog.String("port", cfg.APIPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", slog.Any("error", err))
	}
	logger.Info("server stopped")
}

func runMigrations(databaseURL string, logger *slog.Logger) {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		logger.Error("create migrator failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("run migrations failed", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("migrations applied successfully")
}
