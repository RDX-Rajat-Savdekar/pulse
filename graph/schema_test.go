package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
)

type fakeQuery struct {
	event *querypb.Event
}

func (f fakeQuery) GetEvent(_ context.Context, in *querypb.GetEventRequest, _ ...grpc.CallOption) (*querypb.Event, error) {
	if f.event == nil || in.GetId() != f.event.GetId() {
		return nil, status.Error(codes.NotFound, "event not found")
	}
	return f.event, nil
}

func (f fakeQuery) ListEvents(context.Context, *querypb.ListEventsRequest, ...grpc.CallOption) (*querypb.ListEventsResponse, error) {
	return &querypb.ListEventsResponse{Events: []*querypb.Event{f.event}}, nil
}

func TestEventQueryReturnsTheCachedRecord(t *testing.T) {
	when := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	h := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{QueryClient: fakeQuery{
		event: &querypb.Event{
			Id:               "evt-1",
			Type:             "page.view",
			Source:           "web",
			PayloadJson:      `{"path":"/"}`,
			OccurredAtUnixMs: when.UnixMilli(),
		},
	}}}))

	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"query":"{ event(id: \"evt-1\") { id type source payload occurredAt } }"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Event struct {
				ID, Type, Source, Payload, OccurredAt string
			}
		}
		Errors []struct{ Message string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) > 0 {
		t.Fatalf("errors %+v", body.Errors)
	}
	ev := body.Data.Event
	if ev.ID != "evt-1" || ev.Type != "page.view" || ev.Payload != `{"path":"/"}` {
		t.Fatalf("%+v", ev)
	}
	if ev.OccurredAt != when.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("time %s", ev.OccurredAt)
	}
}

func TestMissingEventIsNull(t *testing.T) {
	h := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{QueryClient: fakeQuery{}}}))
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"query":"{ event(id: \"missing\") { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"event":null`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
