package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/bus"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/cache"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/config"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/httpserver"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := config.Brokers()
	topic := config.String("KAFKA_TOPIC", "pulse.events")
	group := config.String("KAFKA_GROUP", "pulse-processor")
	redisAddr := config.String("REDIS_ADDR", "localhost:6379")
	databaseURL := config.String("DATABASE_URL", "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable")
	metricsAddr := config.String("METRICS_ADDR", ":9102")

	db, err := store.NewPostgres(ctx, databaseURL)
	if err != nil {
		slog.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	rdb := cache.New(redisAddr)
	defer rdb.Close()

	if err := bus.Wait(ctx, "postgres", db.Ping); err != nil {
		slog.Error("postgres", "err", err)
		os.Exit(1)
	}
	if err := bus.Wait(ctx, "redis", rdb.Ping); err != nil {
		slog.Error("redis", "err", err)
		os.Exit(1)
	}
	if err := bus.Wait(ctx, "kafka", func(ctx context.Context) error {
		return bus.Dial(ctx, brokers[0])
	}); err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}

	reader := bus.NewReader(brokers, topic, group)
	defer reader.Close()

	errCh := make(chan error, 1)
	go func() {
		err := bus.Run(ctx, reader, db, rdb)
		if errors.Is(err, context.Canceled) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := httpserver.Run(ctx, metricsAddr, mux); err != nil {
			slog.Error("metrics", "err", err)
		}
	}()

	slog.Info("processor consuming", "topic", topic, "group", group, "metrics", metricsAddr)
	if err := <-errCh; err != nil {
		slog.Error("consumer stopped", "err", err)
		os.Exit(1)
	}
}
