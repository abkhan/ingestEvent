package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	EventsTotal              *prometheus.CounterVec
	RequestDuration          *prometheus.HistogramVec
	AnalyticalFlushBatchSize prometheus.Histogram
	AnalyticalFlushDuration  prometheus.Histogram
	DroppedDuplicates        *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	return &Metrics{
		EventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ingest_events_total",
			Help: "Total number of ingested events",
		}, []string{"tenant", "status"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ingest_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),
		AnalyticalFlushBatchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytical_flush_batch_size",
			Help:    "Batch size for analytical flushes",
			Buckets: prometheus.DefBuckets,
		}),
		AnalyticalFlushDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytical_flush_duration_seconds",
			Help:    "Duration of analytical flushes in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		DroppedDuplicates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dropped_duplicates_total",
			Help: "Total number of dropped duplicate events",
		}, []string{"tenant"}),
	}
}

func (m *Metrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.EventsTotal,
		m.RequestDuration,
		m.AnalyticalFlushBatchSize,
		m.AnalyticalFlushDuration,
		m.DroppedDuplicates,
	}
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}
