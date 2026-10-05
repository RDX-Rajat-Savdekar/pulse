package ingest

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/metrics"
)

// Deduper claims an event id. fresh is true only for the first claim.
type Deduper interface {
	Mark(ctx context.Context, id string) (fresh bool, err error)
	Forget(ctx context.Context, id string) error
}

// Publisher writes one event to the backbone.
type Publisher interface {
	Publish(ctx context.Context, ev event.Event) error
}

// Pinger reports whether a dependency can take traffic.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	Dedup Deduper
	Bus   Publisher
	Ready Pinger
	Now   func() time.Time
}

func Router(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", h.ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/v1/events", h.postEvent)
	return r
}

func (h *Handler) clock() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ready(c *gin.Context) {
	if h.Ready == nil {
		c.String(http.StatusOK, "ok")
		return
	}
	if err := h.Ready.Ping(c.Request.Context()); err != nil {
		c.String(http.StatusServiceUnavailable, "not ready")
		return
	}
	c.String(http.StatusOK, "ok")
}

func (h *Handler) postEvent(c *gin.Context) {
	var ev event.Event
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := ev.Validate(h.clock()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fresh, err := h.Dedup.Mark(c.Request.Context(), ev.ID)
	if err != nil {
		metrics.IngestErrors.Inc()
		slog.Error("dedup", "id", ev.ID, "err", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	if !fresh {
		metrics.Duplicates.Inc()
		c.JSON(http.StatusOK, gin.H{"status": "duplicate", "id": ev.ID})
		return
	}

	if err := h.Bus.Publish(c.Request.Context(), ev); err != nil {
		metrics.IngestErrors.Inc()
		slog.Error("publish", "id", ev.ID, "err", err)
		if forgetErr := h.Dedup.Forget(c.Request.Context(), ev.ID); forgetErr != nil {
			slog.Error("dedup forget", "id", ev.ID, "err", forgetErr)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	metrics.Accepted.Inc()
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "id": ev.ID})
}
