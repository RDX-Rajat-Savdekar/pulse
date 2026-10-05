package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/bus"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/cache"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/config"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/httpserver"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/query"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisAddr := config.String("REDIS_ADDR", "localhost:6379")
	databaseURL := config.String("DATABASE_URL", "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable")
	grpcAddr := config.String("GRPC_ADDR", ":50051")
	metricsAddr := config.String("METRICS_ADDR", ":9103")

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

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
	srv := grpc.NewServer()
	querypb.RegisterQueryServiceServer(srv, query.NewServer(rdb, db))
	reflection.Register(srv)

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if err := db.Ping(r.Context()); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		mux.Handle("/metrics", promhttp.Handler())
		if err := httpserver.Run(ctx, metricsAddr, mux); err != nil {
			slog.Error("metrics", "err", err)
		}
	}()

	slog.Info("query listening", "grpc", grpcAddr, "metrics", metricsAddr)
	if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		slog.Error("grpc", "err", err)
		os.Exit(1)
	}
}
