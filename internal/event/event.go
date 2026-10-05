package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound = errors.New("event not found")
	ErrInvalid  = errors.New("invalid event")
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Event is the payload that moves from ingest through Kafka into storage.
type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurredAt"`
}

func (e Event) Validate(now time.Time) error {
	if !idPattern.MatchString(e.ID) {
		return fmt.Errorf("%w: id", ErrInvalid)
	}
	if e.Type == "" || utf8.RuneCountInString(e.Type) > 64 {
		return fmt.Errorf("%w: type", ErrInvalid)
	}
	if e.Source == "" || utf8.RuneCountInString(e.Source) > 64 {
		return fmt.Errorf("%w: source", ErrInvalid)
	}
	if len(e.Payload) == 0 || len(e.Payload) > 64*1024 || !json.Valid(e.Payload) || string(e.Payload) == "null" {
		return fmt.Errorf("%w: payload", ErrInvalid)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurredAt", ErrInvalid)
	}
	if e.OccurredAt.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("%w: occurredAt in the future", ErrInvalid)
	}
	return nil
}

func (e Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func Unmarshal(b []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(b, &e); err != nil {
		return Event{}, fmt.Errorf("unmarshal event: %w", err)
	}
	return e, nil
}
