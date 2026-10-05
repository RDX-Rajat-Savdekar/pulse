package query

import (
	"context"
	"log/slog"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/metrics"
)

// Store is the durable read path.
type Store interface {
	Get(ctx context.Context, id string) (event.Event, error)
	List(ctx context.Context, limit int) ([]event.Event, error)
}

// HotCache serves recent events. ok is false when the cache cannot answer.
type HotCache interface {
	Get(ctx context.Context, id string) (event.Event, bool, error)
	Put(ctx context.Context, ev event.Event) error
	Recent(ctx context.Context, limit int) ([]event.Event, bool, error)
}

func ClampLimit(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

func ReadOne(ctx context.Context, id string, cache HotCache, store Store) (event.Event, error) {
	ev, ok, err := cache.Get(ctx, id)
	if err != nil {
		return event.Event{}, err
	}
	if ok {
		metrics.CacheHits.Inc()
		return ev, nil
	}
	metrics.CacheMisses.Inc()
	ev, err = store.Get(ctx, id)
	if err != nil {
		return event.Event{}, err
	}
	if putErr := cache.Put(ctx, ev); putErr != nil {
		slog.Warn("cache backfill failed", "id", id, "err", putErr)
	}
	return ev, nil
}

func ReadMany(ctx context.Context, limit int, cache HotCache, store Store) ([]event.Event, error) {
	limit = ClampLimit(limit)
	evs, ok, err := cache.Recent(ctx, limit)
	if err != nil {
		return nil, err
	}
	if ok {
		metrics.CacheHits.Inc()
		return evs, nil
	}
	metrics.CacheMisses.Inc()
	return store.List(ctx, limit)
}
