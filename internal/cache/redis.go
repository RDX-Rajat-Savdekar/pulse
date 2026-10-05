package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

const (
	dedupPrefix = "pulse:dedup:"
	hotPrefix   = "pulse:hot:"
	recentKey   = "pulse:recent"
	dedupTTL    = 24 * time.Hour
	hotTTL      = 15 * time.Minute
	recentMax   = 100
)

type Redis struct {
	rdb *redis.Client
}

func New(addr string) *Redis {
	return &Redis{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

func (c *Redis) Close() error { return c.rdb.Close() }

func (c *Redis) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

func (c *Redis) Mark(ctx context.Context, id string) (bool, error) {
	ok, err := c.rdb.SetNX(ctx, dedupPrefix+id, "1", dedupTTL).Result()
	if err != nil {
		return false, fmt.Errorf("dedup mark: %w", err)
	}
	return ok, nil
}

func (c *Redis) Forget(ctx context.Context, id string) error {
	if err := c.rdb.Del(ctx, dedupPrefix+id).Err(); err != nil {
		return fmt.Errorf("dedup forget: %w", err)
	}
	return nil
}

func (c *Redis) Put(ctx context.Context, ev event.Event) error {
	body, err := ev.Marshal()
	if err != nil {
		return err
	}
	pipe := c.rdb.TxPipeline()
	pipe.Set(ctx, hotPrefix+ev.ID, body, hotTTL)
	pipe.LRem(ctx, recentKey, 0, ev.ID)
	pipe.LPush(ctx, recentKey, ev.ID)
	pipe.LTrim(ctx, recentKey, 0, recentMax-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache put: %w", err)
	}
	return nil
}

func (c *Redis) Get(ctx context.Context, id string) (event.Event, bool, error) {
	raw, err := c.rdb.Get(ctx, hotPrefix+id).Bytes()
	if err == redis.Nil {
		return event.Event{}, false, nil
	}
	if err != nil {
		return event.Event{}, false, fmt.Errorf("cache get: %w", err)
	}
	ev, err := event.Unmarshal(raw)
	if err != nil {
		return event.Event{}, false, err
	}
	return ev, true, nil
}

func (c *Redis) Recent(ctx context.Context, limit int) ([]event.Event, bool, error) {
	if limit <= 0 {
		return nil, false, nil
	}
	ids, err := c.rdb.LRange(ctx, recentKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, false, fmt.Errorf("cache recent: %w", err)
	}
	if len(ids) == 0 {
		return nil, false, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = hotPrefix + id
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, fmt.Errorf("cache mget: %w", err)
	}
	out := make([]event.Event, 0, len(vals))
	for _, v := range vals {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, false, nil
		}
		ev, err := event.Unmarshal([]byte(s))
		if err != nil {
			return nil, false, err
		}
		out = append(out, ev)
	}
	return out, true, nil
}
