package instrumentation

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	errorsTotal     *prometheus.CounterVec
	requestsActive  prometheus.Gauge
}

func NewMetrics(pluginID string) *Metrics {
	m := &Metrics{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "grafana_plugin",
				Subsystem: pluginID,
				Name:      "request_duration_seconds",
				Help:      "Request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation", "status"},
		),
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "grafana_plugin",
				Subsystem: pluginID,
				Name:      "requests_total",
				Help:      "Total number of requests.",
			},
			[]string{"operation"},
		),
		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "grafana_plugin",
				Subsystem: pluginID,
				Name:      "errors_total",
				Help:      "Total number of errors.",
			},
			[]string{"operation", "error_type"},
		),
		requestsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "grafana_plugin",
				Subsystem: pluginID,
				Name:      "requests_active",
				Help:      "Current number of active requests.",
			},
		),
	}

	prometheus.MustRegister(
		m.requestDuration,
		m.requestsTotal,
		m.errorsTotal,
		m.requestsActive,
	)

	return m
}

// RecordRequest records metrics for a request
func (m *Metrics) RecordRequest(operation string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		m.errorsTotal.WithLabelValues(operation, err.Error()).Inc()
	}
	m.requestDuration.WithLabelValues(operation, status).Observe(duration)
	m.requestsTotal.WithLabelValues(operation).Inc()
}
