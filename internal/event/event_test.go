package event

import (
	"strings"
	"testing"
	"time"
)

func valid() Event {
	return Event{
		ID:         "evt-1",
		Type:       "page.view",
		Source:     "web",
		Payload:    []byte(`{"path":"/"}`),
		OccurredAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
	}
}

func TestValidateAcceptsANormalEvent(t *testing.T) {
	if err := valid().Validate(time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsBadIDTypePayloadAndFutureTime(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	cases := []Event{
		func() Event { e := valid(); e.ID = "has space"; return e }(),
		func() Event { e := valid(); e.Type = ""; return e }(),
		func() Event { e := valid(); e.Source = strings.Repeat("s", 65); return e }(),
		func() Event { e := valid(); e.Payload = []byte(`null`); return e }(),
		func() Event { e := valid(); e.Payload = []byte(`{`); return e }(),
		func() Event { e := valid(); e.OccurredAt = now.Add(time.Hour); return e }(),
		func() Event { e := valid(); e.OccurredAt = time.Time{}; return e }(),
	}
	for i, ev := range cases {
		if err := ev.Validate(now); err == nil {
			t.Fatalf("case %d: expected invalid", i)
		}
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	in := valid()
	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	out, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID || out.Type != in.Type || string(out.Payload) != string(in.Payload) {
		t.Fatalf("got %+v", out)
	}
	if !out.OccurredAt.Equal(in.OccurredAt) {
		t.Fatalf("time %s", out.OccurredAt)
	}
}
