package pipeline

import (
	"context"
	"log/slog"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

// Store is the durable write path. Insert is idempotent.
type Store interface {
	Insert(ctx context.Context, ev event.Event) (inserted bool, err error)
}

// Cache is the hot-read side. A cache failure must not block the durable write.
type Cache interface {
	Put(ctx context.Context, ev event.Event) error
}

func Apply(ctx context.Context, ev event.Event, store Store, cache Cache) error {
	if _, err := store.Insert(ctx, ev); err != nil {
		return err
	}
	if err := cache.Put(ctx, ev); err != nil {
		slog.Warn("hot cache put failed", "id", ev.ID, "err", err)
	}
	return nil
}
