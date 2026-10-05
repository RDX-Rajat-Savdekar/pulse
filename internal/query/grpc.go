package query

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
)

type Server struct {
	querypb.UnimplementedQueryServiceServer
	cache HotCache
	store Store
}

func NewServer(cache HotCache, store Store) *Server {
	return &Server{cache: cache, store: store}
}

func (s *Server) GetEvent(ctx context.Context, req *querypb.GetEventRequest) (*querypb.Event, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	ev, err := ReadOne(ctx, req.GetId(), s.cache, s.store)
	if errors.Is(err, event.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "event not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get event: %v", err)
	}
	return toProto(ev), nil
}

func (s *Server) ListEvents(ctx context.Context, req *querypb.ListEventsRequest) (*querypb.ListEventsResponse, error) {
	evs, err := ReadMany(ctx, int(req.GetLimit()), s.cache, s.store)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list events: %v", err)
	}
	out := &querypb.ListEventsResponse{Events: make([]*querypb.Event, 0, len(evs))}
	for _, ev := range evs {
		out.Events = append(out.Events, toProto(ev))
	}
	return out, nil
}

func toProto(ev event.Event) *querypb.Event {
	return &querypb.Event{
		Id:               ev.ID,
		Type:             ev.Type,
		Source:           ev.Source,
		PayloadJson:      string(ev.Payload),
		OccurredAtUnixMs: ev.OccurredAt.UnixMilli(),
	}
}
