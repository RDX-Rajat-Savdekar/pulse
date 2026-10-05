package bus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

type script struct {
	values  [][]byte
	commits int
	i       int
}

func (s *script) Next(context.Context) ([]byte, func(context.Context) error, error) {
	if s.i >= len(s.values) {
		return nil, nil, errors.New("done")
	}
	v := s.values[s.i]
	s.i++
	return v, func(context.Context) error {
		s.commits++
		return nil
	}, nil
}

type store struct {
	ids  []string
	fail bool
}

func (s *store) Insert(_ context.Context, ev event.Event) (bool, error) {
	if s.fail {
		return false, errors.New("db down")
	}
	s.ids = append(s.ids, ev.ID)
	return true, nil
}

type cache struct{}

func (cache) Put(context.Context, event.Event) error { return nil }

func TestRunCommitsPoisonAndPersistsGoodRecords(t *testing.T) {
	good, err := (event.Event{
		ID:         "evt-1",
		Type:       "page.view",
		Source:     "web",
		Payload:    []byte(`{"path":"/"}`),
		OccurredAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	src := &script{values: [][]byte{[]byte("{"), good}}
	st := &store{}
	err = Run(context.Background(), src, st, cache{})
	if err == nil || err.Error() != "done" {
		t.Fatalf("err %v", err)
	}
	if src.commits != 2 {
		t.Fatalf("commits %d", src.commits)
	}
	if len(st.ids) != 1 || st.ids[0] != "evt-1" {
		t.Fatalf("stored %+v", st.ids)
	}
}

func TestRunDoesNotCommitWhenStoreFails(t *testing.T) {
	good, err := (event.Event{
		ID:         "evt-1",
		Type:       "page.view",
		Source:     "web",
		Payload:    []byte(`{}`),
		OccurredAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	src := &script{values: [][]byte{good}}
	err = Run(context.Background(), src, &store{fail: true}, cache{})
	if err == nil {
		t.Fatal("expected store error")
	}
	if src.commits != 0 {
		t.Fatalf("committed a failed record: %d", src.commits)
	}
}
