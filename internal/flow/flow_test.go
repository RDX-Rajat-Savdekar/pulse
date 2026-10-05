package flow

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/ingest"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/pipeline"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/query"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
)

func TestAcceptedEventSurvivesDedupAndIsReadableOverGRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dedup := &memDedup{}
	pub := &recBus{}
	router := ingest.Router(&ingest.Handler{
		Dedup: dedup,
		Bus:   pub,
		Now: func() time.Time {
			return time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC)
		},
	})
	raw := `{"id":"evt-1","type":"page.view","source":"web","payload":{"path":"/"},"occurredAt":"2026-10-04T12:00:00Z"}`
	if code := post(router, raw); code != http.StatusAccepted {
		t.Fatalf("accept %d", code)
	}
	if code := post(router, raw); code != http.StatusOK {
		t.Fatalf("duplicate %d", code)
	}
	if len(pub.events) != 1 {
		t.Fatalf("published %d", len(pub.events))
	}

	st := &memStore{one: map[string]event.Event{}}
	cache := &memCache{items: map[string]event.Event{}}
	if err := pipeline.Apply(context.Background(), pub.events[0], st, cache); err != nil {
		t.Fatal(err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	querypb.RegisterQueryServiceServer(srv, query.NewServer(cache, st))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := querypb.NewQueryServiceClient(conn)
	got, err := client.GetEvent(context.Background(), &querypb.GetEventRequest{Id: "evt-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetType() != "page.view" || got.GetPayloadJson() != `{"path":"/"}` {
		t.Fatalf("%+v", got)
	}

	delete(st.one, "evt-1")
	got, err = client.GetEvent(context.Background(), &querypb.GetEventRequest{Id: "evt-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetSource() != "web" {
		t.Fatalf("cache miss after store delete: %+v", got)
	}
}

func post(h http.Handler, body string) int {
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

type memDedup struct{ seen map[string]struct{} }

func (d *memDedup) Mark(_ context.Context, id string) (bool, error) {
	if d.seen == nil {
		d.seen = map[string]struct{}{}
	}
	if _, ok := d.seen[id]; ok {
		return false, nil
	}
	d.seen[id] = struct{}{}
	return true, nil
}

func (d *memDedup) Forget(_ context.Context, id string) error {
	delete(d.seen, id)
	return nil
}

type recBus struct{ events []event.Event }

func (b *recBus) Publish(_ context.Context, ev event.Event) error {
	b.events = append(b.events, ev)
	return nil
}

type memStore struct{ one map[string]event.Event }

func (m *memStore) Insert(_ context.Context, ev event.Event) (bool, error) {
	m.one[ev.ID] = ev
	return true, nil
}

func (m *memStore) Get(_ context.Context, id string) (event.Event, error) {
	ev, ok := m.one[id]
	if !ok {
		return event.Event{}, event.ErrNotFound
	}
	return ev, nil
}

func (m *memStore) List(context.Context, int) ([]event.Event, error) { return nil, nil }

type memCache struct{ items map[string]event.Event }

func (m *memCache) Put(_ context.Context, ev event.Event) error {
	m.items[ev.ID] = ev
	return nil
}

func (m *memCache) Get(_ context.Context, id string) (event.Event, bool, error) {
	ev, ok := m.items[id]
	return ev, ok, nil
}

func (m *memCache) Recent(context.Context, int) ([]event.Event, bool, error) {
	return nil, false, nil
}
