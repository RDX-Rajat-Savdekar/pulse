package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RDX-Rajat-Savdekar/pulse/internal/event"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) Insert(ctx context.Context, ev event.Event) (bool, error) {
	tag, err := p.pool.Exec(ctx, `
		INSERT INTO events (id, type, source, payload, occurred_at)
		VALUES ($1, $2, $3, $4::jsonb, $5)
		ON CONFLICT (id) DO NOTHING
	`, ev.ID, ev.Type, ev.Source, string(ev.Payload), ev.OccurredAt)
	if err != nil {
		return false, fmt.Errorf("insert event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (p *Postgres) Get(ctx context.Context, id string) (event.Event, error) {
	var ev event.Event
	var payload []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, type, source, payload, occurred_at
		FROM events WHERE id = $1
	`, id).Scan(&ev.ID, &ev.Type, &ev.Source, &payload, &ev.OccurredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return event.Event{}, event.ErrNotFound
	}
	if err != nil {
		return event.Event{}, fmt.Errorf("get event: %w", err)
	}
	ev.Payload = payload
	return ev, nil
}

func (p *Postgres) List(ctx context.Context, limit int) ([]event.Event, error) {
	limit = clampLimit(limit)
	rows, err := p.pool.Query(ctx, `
		SELECT id, type, source, payload, occurred_at
		FROM events
		ORDER BY occurred_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var out []event.Event
	for rows.Next() {
		var ev event.Event
		var payload []byte
		if err := rows.Scan(&ev.ID, &ev.Type, &ev.Source, &payload, &ev.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.Payload = payload
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func clampLimit(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}
