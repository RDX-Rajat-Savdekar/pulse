package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

type memStore struct {
	err  error
	got  []event.Event
	fail bool
}

func (m *memStore) Insert(_ context.Context, ev event.Event) (bool, error) {
	if m.fail {
		return false, errors.New("db down")
	}
	m.got = append(m.got, ev)
	return true, nil
}

type memCache struct {
	err error
	got []event.Event
}

func (m *memCache) Put(_ context.Context, ev event.Event) error {
	if m.err != nil {
		return m.err
	}
	m.got = append(m.got, ev)
	return nil
}

func TestApplyWritesStoreThenCache(t *testing.T) {
	ev := event.Event{ID: "evt-1"}
	store := &memStore{}
	cache := &memCache{}
	if err := Apply(context.Background(), ev, store, cache); err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 1 || store.got[0].ID != "evt-1" {
		t.Fatalf("store %+v", store.got)
	}
	if len(cache.got) != 1 {
		t.Fatalf("cache %+v", cache.got)
	}
}

func TestApplyReturnsStoreError(t *testing.T) {
	store := &memStore{fail: true}
	cache := &memCache{}
	if err := Apply(context.Background(), event.Event{ID: "evt-1"}, store, cache); err == nil {
		t.Fatal("expected store error")
	}
	if len(cache.got) != 0 {
		t.Fatal("cache was written after a failed insert")
	}
}

func TestApplyKeepsGoingWhenCacheFails(t *testing.T) {
	store := &memStore{}
	cache := &memCache{err: errors.New("redis down")}
	if err := Apply(context.Background(), event.Event{ID: "evt-1"}, store, cache); err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 1 {
		t.Fatal("store write was lost")
	}
}
