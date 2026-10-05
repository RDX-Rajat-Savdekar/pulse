package query

import (
	"context"
	"testing"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

type memStore struct {
	one  map[string]event.Event
	list []event.Event
}

func (m *memStore) Get(_ context.Context, id string) (event.Event, error) {
	ev, ok := m.one[id]
	if !ok {
		return event.Event{}, event.ErrNotFound
	}
	return ev, nil
}

func (m *memStore) List(_ context.Context, limit int) ([]event.Event, error) {
	if limit > len(m.list) {
		limit = len(m.list)
	}
	return append([]event.Event(nil), m.list[:limit]...), nil
}

type memCache struct {
	items  map[string]event.Event
	recent []event.Event
	full   bool
}

func (m *memCache) Get(_ context.Context, id string) (event.Event, bool, error) {
	ev, ok := m.items[id]
	return ev, ok, nil
}

func (m *memCache) Put(_ context.Context, ev event.Event) error {
	if m.items == nil {
		m.items = map[string]event.Event{}
	}
	m.items[ev.ID] = ev
	return nil
}

func (m *memCache) Recent(_ context.Context, limit int) ([]event.Event, bool, error) {
	if !m.full {
		return nil, false, nil
	}
	if limit > len(m.recent) {
		limit = len(m.recent)
	}
	return append([]event.Event(nil), m.recent[:limit]...), true, nil
}

func TestReadOneUsesCacheWithoutStore(t *testing.T) {
	cache := &memCache{items: map[string]event.Event{"evt-1": {ID: "evt-1", Type: "cached"}}}
	store := &memStore{}
	got, err := ReadOne(context.Background(), "evt-1", cache, store)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "cached" {
		t.Fatalf("type %s", got.Type)
	}
}

func TestReadOneFallsBackAndBackfills(t *testing.T) {
	cache := &memCache{}
	store := &memStore{one: map[string]event.Event{"evt-1": {ID: "evt-1", Type: "stored"}}}
	got, err := ReadOne(context.Background(), "evt-1", cache, store)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "stored" {
		t.Fatalf("type %s", got.Type)
	}
	if _, ok := cache.items["evt-1"]; !ok {
		t.Fatal("expected cache backfill")
	}
}

func TestReadManyUsesRecentCache(t *testing.T) {
	cache := &memCache{full: true, recent: []event.Event{{ID: "a"}, {ID: "b"}}}
	store := &memStore{list: []event.Event{{ID: "from-db"}}}
	got, err := ReadMany(context.Background(), 1, cache, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("%+v", got)
	}
}

func TestReadManyFallsBackWhenCacheIsCold(t *testing.T) {
	cache := &memCache{}
	store := &memStore{list: []event.Event{{ID: "from-db"}}}
	got, err := ReadMany(context.Background(), 0, cache, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "from-db" {
		t.Fatalf("%+v", got)
	}
}

func TestClampLimit(t *testing.T) {
	if ClampLimit(0) != 20 || ClampLimit(500) != 100 || ClampLimit(7) != 7 {
		t.Fatalf("clamp %d %d %d", ClampLimit(0), ClampLimit(500), ClampLimit(7))
	}
}
