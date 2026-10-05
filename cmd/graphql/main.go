package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RDX-Rajat-Savdekar/pulse/graph"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/config"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/httpserver"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
	"github.com/RDX-Rajat-Savdekar/pulse/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpAddr := config.String("HTTP_ADDR", ":8081")
	queryAddr := config.String("QUERY_GRPC_ADDR", "localhost:50051")

	conn, err := grpc.NewClient(queryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("query dial", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	gql := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{QueryClient: querypb.NewQueryServiceClient(conn)},
	}))
	mux := http.NewServeMux()
	mux.Handle("/query", gql)
	mux.Handle("/playground", playground.Handler("Pulse", "/query"))
	mux.Handle("POST /v1/events", proxyIngest(config.String("INGEST_URL", "http://127.0.0.1:8080")))
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", web.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		dialer := net.Dialer{Timeout: time.Second}
		c, err := dialer.DialContext(r.Context(), "tcp", queryAddr)
		if err != nil {
			http.Error(w, "query unavailable", http.StatusServiceUnavailable)
			return
		}
		c.Close()
		w.WriteHeader(http.StatusOK)
	})

	slog.Info("graphql listening", "addr", httpAddr, "query", queryAddr)
	if err := httpserver.Run(ctx, httpAddr, mux); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}

func proxyIngest(base string) http.Handler {
	client := &http.Client{Timeout: 5 * time.Second}
	endpoint := strings.TrimRight(base, "/") + "/v1/events"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 70*1024))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "ingest unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
}
