package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/bus"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/cache"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/config"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/httpserver"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/ingest"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := config.Brokers()
	topic := config.String("KAFKA_TOPIC", "pulse.events")
	redisAddr := config.String("REDIS_ADDR", "localhost:6379")
	httpAddr := config.String("HTTP_ADDR", ":8080")

	rdb := cache.New(redisAddr)
	defer rdb.Close()
	producer := bus.NewProducer(brokers, topic)
	defer producer.Close()

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

	router := ingest.Router(&ingest.Handler{Dedup: rdb, Bus: producer, Ready: rdb})
	slog.Info("ingest listening", "addr", httpAddr)
	if err := httpserver.Run(ctx, httpAddr, router); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}
