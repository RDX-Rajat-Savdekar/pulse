package graph

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RDX-Rajat-Savdekar/pulse/graph/model"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/querypb"
)

func (r *queryResolver) Event(ctx context.Context, id string) (*model.Event, error) {
	resp, err := r.QueryClient.GetEvent(ctx, &querypb.GetEventRequest{Id: id})
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromProto(resp), nil
}

func (r *queryResolver) Events(ctx context.Context, limit *int) ([]*model.Event, error) {
	req := &querypb.ListEventsRequest{}
	if limit != nil {
		req.Limit = int32(*limit)
	}
	resp, err := r.QueryClient.ListEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Event, 0, len(resp.GetEvents()))
	for _, ev := range resp.GetEvents() {
		out = append(out, fromProto(ev))
	}
	return out, nil
}

func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }

func fromProto(ev *querypb.Event) *model.Event {
	return &model.Event{
		ID:         ev.GetId(),
		Type:       ev.GetType(),
		Source:     ev.GetSource(),
		Payload:    ev.GetPayloadJson(),
		OccurredAt: time.UnixMilli(ev.GetOccurredAtUnixMs()).UTC().Format(time.RFC3339Nano),
	}
}
