package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	Accepted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_accepted_total",
		Help: "Events accepted onto Kafka.",
	})
	Duplicates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_duplicate_total",
		Help: "Events rejected by the Redis dedup gate.",
	})
	IngestErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_ingest_errors_total",
		Help: "Ingest failures after validation.",
	})
	Processed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_processed_total",
		Help: "Events committed after a successful store write.",
	})
	ProcessErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_process_errors_total",
		Help: "Consumer failures that leave the offset uncommitted.",
	})
	Poison = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_events_poison_total",
		Help: "Undecodable Kafka records that were skipped.",
	})
	ProcessSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pulse_event_process_seconds",
		Help:    "Time to persist one event and update the hot cache.",
		Buckets: prometheus.DefBuckets,
	})
	CacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_query_cache_hits_total",
		Help: "Query reads served from Redis.",
	})
	CacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_query_cache_misses_total",
		Help: "Query reads that fell through to PostgreSQL.",
	})
)
