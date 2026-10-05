package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type memDedup struct {
	seen  map[string]struct{}
	err   error
	marks int
}

func (d *memDedup) Mark(_ context.Context, id string) (bool, error) {
	if d.err != nil {
		return false, d.err
	}
	d.marks++
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

type recBus struct {
	events []event.Event
	err    error
}

func (b *recBus) Publish(_ context.Context, ev event.Event) error {
	if b.err != nil {
		return b.err
	}
	b.events = append(b.events, ev)
	return nil
}

func body() string {
	return `{"id":"evt-1","type":"page.view","source":"web","payload":{"path":"/"},"occurredAt":"2026-10-04T12:00:00Z"}`
}

func post(h http.Handler, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestPostEventPublishesOnceAndReportsDuplicate(t *testing.T) {
	dedup := &memDedup{}
	bus := &recBus{}
	h := Router(&Handler{Dedup: dedup, Bus: bus, Now: func() time.Time {
		return time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC)
	}})

	first := post(h, body())
	if first.Code != http.StatusAccepted {
		t.Fatalf("first %d %s", first.Code, first.Body.String())
	}
	second := post(h, body())
	if second.Code != http.StatusOK {
		t.Fatalf("second %d %s", second.Code, second.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "duplicate" || len(bus.events) != 1 {
		t.Fatalf("status %s published %d", payload["status"], len(bus.events))
	}
}

func TestPostEventReleasesDedupWhenPublishFails(t *testing.T) {
	dedup := &memDedup{}
	bus := &recBus{err: errors.New("kafka down")}
	h := Router(&Handler{Dedup: dedup, Bus: bus, Now: func() time.Time {
		return time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC)
	}})
	w := post(h, body())
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", w.Code)
	}
	if _, stuck := dedup.seen["evt-1"]; stuck {
		t.Fatal("dedup key left behind after publish failure")
	}

	bus.err = nil
	w = post(h, body())
	if w.Code != http.StatusAccepted {
		t.Fatalf("retry %d %s", w.Code, w.Body.String())
	}
	if len(bus.events) != 1 {
		t.Fatalf("published %d", len(bus.events))
	}
}

func TestPostEventRejectsInvalidJSONAndFutureEvents(t *testing.T) {
	h := Router(&Handler{Dedup: &memDedup{}, Bus: &recBus{}, Now: func() time.Time {
		return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	}})
	if post(h, `{`).Code != http.StatusBadRequest {
		t.Fatal("expected bad json")
	}
	future := `{"id":"evt-2","type":"page.view","source":"web","payload":{},"occurredAt":"2026-10-04T13:00:00Z"}`
	if post(h, future).Code != http.StatusBadRequest {
		t.Fatal("expected future rejection")
	}
}

func TestReadyDependsOnPing(t *testing.T) {
	h := Router(&Handler{Dedup: &memDedup{}, Bus: &recBus{}, Ready: pingErr{}})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", w.Code)
	}
}

type pingErr struct{}

func (pingErr) Ping(context.Context) error { return errors.New("down") }
