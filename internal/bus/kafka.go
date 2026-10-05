package bus

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/metrics"
	"github.com/RDX-Rajat-Savdekar/pulse/internal/pipeline"
)

// Producer publishes JSON events. Writer fields follow segmentio/kafka-go:
// https://pkg.go.dev/github.com/segmentio/kafka-go#Writer
type Producer struct {
	w *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{w: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}}
}

func (p *Producer) Publish(ctx context.Context, ev event.Event) error {
	body, err := ev.Marshal()
	if err != nil {
		return err
	}
	return p.w.WriteMessages(ctx, kafka.Message{Key: []byte(ev.ID), Value: body})
}

func (p *Producer) Close() error { return p.w.Close() }

// Source yields one record and a commit function for that record.
type Source interface {
	Next(ctx context.Context) (value []byte, commit func(context.Context) error, err error)
}

type Reader struct {
	r *kafka.Reader
}

func NewReader(brokers []string, topic, group string) *Reader {
	return &Reader{r: kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
	})}
}

func (r *Reader) Next(ctx context.Context) ([]byte, func(context.Context) error, error) {
	msg, err := r.r.FetchMessage(ctx)
	if err != nil {
		return nil, nil, err
	}
	commit := func(ctx context.Context) error {
		return r.r.CommitMessages(ctx, msg)
	}
	return msg.Value, commit, nil
}

func (r *Reader) Close() error { return r.r.Close() }

func Dial(ctx context.Context, broker string) error {
	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	return conn.Close()
}

// Run reads until ctx is canceled. Poison records are committed and skipped.
// A store failure returns before commit so Kafka redelivers.
func Run(ctx context.Context, src Source, store pipeline.Store, cache pipeline.Cache) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		value, commit, err := src.Next(ctx)
		if err != nil {
			return err
		}
		ev, err := event.Unmarshal(value)
		if err != nil {
			metrics.Poison.Inc()
			slog.Error("skip poison message", "err", err)
			if err := commit(ctx); err != nil {
				return err
			}
			continue
		}
		start := time.Now()
		if err := pipeline.Apply(ctx, ev, store, cache); err != nil {
			metrics.ProcessErrors.Inc()
			return err
		}
		metrics.Processed.Inc()
		metrics.ProcessSeconds.Observe(time.Since(start).Seconds())
		if err := commit(ctx); err != nil {
			return err
		}
	}
}

func Wait(ctx context.Context, name string, fn func(context.Context) error) error {
	backoff := 200 * time.Millisecond
	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		slog.Warn("waiting", "dep", name, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}
