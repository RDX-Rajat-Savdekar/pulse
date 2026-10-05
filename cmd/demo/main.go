package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/config"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/httpserver"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handler, shutdown, err := newHandler()
	if err != nil {
		slog.Error("demo", "err", err)
		os.Exit(1)
	}
	defer shutdown()

	addr := config.String("HTTP_ADDR", ":8088")
	slog.Info("pulse console", "addr", addr)
	if err := httpserver.Run(ctx, addr, handler); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}
