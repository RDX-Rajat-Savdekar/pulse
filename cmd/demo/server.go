package main

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RDX-Rajat-Savdekar/pulse/graph"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/ingest"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/pipeline"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/query"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
	"github.com/RDX-Rajat-Savdekar/pulse/web"
)

func newHandler() (http.Handler, func(), error) {
	store := newMemStore()
	cache := newMemCache()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	grpcServer := grpc.NewServer()
	querypb.RegisterQueryServiceServer(grpcServer, query.NewServer(cache, store))
	go grpcServer.Serve(lis)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		grpcServer.Stop()
		return nil, nil, err
	}
	gql := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{QueryClient: querypb.NewQueryServiceClient(conn)},
	}))
	ingestRouter := ingest.Router(&ingest.Handler{
		Dedup: newMemDedup(),
		Bus:   &applyBus{store: store, cache: cache},
	})

	mux := http.NewServeMux()
	mux.Handle("/query", gql)
	mux.Handle("/playground", playground.Handler("Pulse", "/query"))
	mux.Handle("POST /v1/events", ingestRouter)
	mux.Handle("/", web.Handler())

	stop := func() {
		conn.Close()
		grpcServer.Stop()
		lis.Close()
	}
	return mux, stop, nil
}

type applyBus struct {
	store *memStore
	cache *memCache
}

func (b *applyBus) Publish(ctx context.Context, ev event.Event) error {
	return pipeline.Apply(ctx, ev, b.store, b.cache)
}

type memDedup struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func newMemDedup() *memDedup { return &memDedup{seen: map[string]struct{}{}} }

func (d *memDedup) Mark(_ context.Context, id string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[id]; ok {
		return false, nil
	}
	d.seen[id] = struct{}{}
	return true, nil
}

func (d *memDedup) Forget(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.seen, id)
	return nil
}

type memStore struct {
	mu    sync.Mutex
	one   map[string]event.Event
	order []event.Event
}

func newMemStore() *memStore { return &memStore{one: map[string]event.Event{}} }

func (m *memStore) Insert(_ context.Context, ev event.Event) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.one[ev.ID]; ok {
		return false, nil
	}
	m.one[ev.ID] = ev
	m.order = append([]event.Event{ev}, m.order...)
	return true, nil
}

func (m *memStore) Get(_ context.Context, id string) (event.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev, ok := m.one[id]
	if !ok {
		return event.Event{}, event.ErrNotFound
	}
	return ev, nil
}

func (m *memStore) List(_ context.Context, limit int) ([]event.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.order) {
		limit = len(m.order)
	}
	return append([]event.Event(nil), m.order[:limit]...), nil
}

type memCache struct {
	mu     sync.Mutex
	items  map[string]event.Event
	recent []event.Event
}

func newMemCache() *memCache { return &memCache{items: map[string]event.Event{}} }

func (m *memCache) Put(_ context.Context, ev event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[ev.ID] = ev
	kept := make([]event.Event, 0, len(m.recent))
	for _, item := range m.recent {
		if item.ID != ev.ID {
			kept = append(kept, item)
		}
	}
	m.recent = append([]event.Event{ev}, kept...)
	return nil
}

func (m *memCache) Get(_ context.Context, id string) (event.Event, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev, ok := m.items[id]
	return ev, ok, nil
}

func (m *memCache) Recent(_ context.Context, limit int) ([]event.Event, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recent) == 0 {
		return nil, false, nil
	}
	if limit > len(m.recent) {
		limit = len(m.recent)
	}
	return append([]event.Event(nil), m.recent[:limit]...), true, nil
}
