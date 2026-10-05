package query

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
)

func TestGetEventOverGRPCUsesCache(t *testing.T) {
	when := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	cache := &memCache{items: map[string]event.Event{
		"evt-1": {ID: "evt-1", Type: "page.view", Source: "web", Payload: []byte(`{"path":"/"}`), OccurredAt: when},
	}}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	querypb.RegisterQueryServiceServer(srv, NewServer(cache, &memStore{}))
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
	if got.GetId() != "evt-1" || got.GetType() != "page.view" || got.GetPayloadJson() != `{"path":"/"}` {
		t.Fatalf("%+v", got)
	}
	if got.GetOccurredAtUnixMs() != when.UnixMilli() {
		t.Fatalf("time %d", got.GetOccurredAtUnixMs())
	}

	_, err = client.GetEvent(context.Background(), &querypb.GetEventRequest{Id: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code %s err %v", status.Code(err), err)
	}
}
