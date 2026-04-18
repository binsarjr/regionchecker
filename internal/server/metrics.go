package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics bundles the Prometheus collectors used by the HTTP server.
type Metrics struct {
	Lookups        *prometheus.CounterVec
	LookupDuration *prometheus.HistogramVec
	HTTPRequests   *prometheus.CounterVec
	HTTPDuration   *prometheus.HistogramVec
	DBAgeSeconds   prometheus.Gauge
}

// NewMetrics registers and returns the server-side metric collectors.
// Registers on prometheus.DefaultRegisterer.
func NewMetrics() *Metrics {
	f := promauto.With(prometheus.DefaultRegisterer)
	return &Metrics{
		Lookups: f.NewCounterVec(prometheus.CounterOpts{
			Name: "regionchecker_lookups_total",
			Help: "Total classifier lookups by result and input type.",
		}, []string{"result", "type"}),
		LookupDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "regionchecker_lookup_duration_seconds",
			Help:    "Classifier lookup latency.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 14),
		}, []string{"type"}),
		HTTPRequests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "regionchecker_http_requests_total",
			Help: "HTTP requests by path and status code.",
		}, []string{"path", "code"}),
		HTTPDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "regionchecker_http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"path"}),
		DBAgeSeconds: f.NewGauge(prometheus.GaugeOpts{
			Name: "regionchecker_db_age_seconds",
			Help: "Age of the parsed RIR snapshot in seconds.",
		}),
	}
}
