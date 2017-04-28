package router

import (
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	selectedBackends      prometheus.Gauge
	retriesByBackend      *prometheus.CounterVec
	requestsByBackend     *prometheus.CounterVec
	responseTimeByBackend *prometheus.CounterVec
	selectionEvents       prometheus.Counter
	affinityHits          *prometheus.CounterVec
}

func newMetrics(metricsNamespace string) *metrics {

	m := &metrics{
		selectedBackends: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "selected_backends",
			Help:      "The number of currently selected backends",
		}),
		retriesByBackend: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "retries",
			Help:      "The number of http request retries",
		}, []string{"backend"}),
		requestsByBackend: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "requests",
			Help:      "The number of requests",
		}, []string{"backend"}),
		responseTimeByBackend: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "response_milliseconds",
			Help:      "The number of milliseconds taken to respond",
		}, []string{"backend"}),
		selectionEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "selection_events",
			Help:      "The number of selection events",
		}),
		affinityHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "affinity_hits",
			Help:      "The number of requests routed based on affinity match",
		}, []string{"type"}),
	}
	prometheus.MustRegister(m.selectedBackends)
	prometheus.MustRegister(m.retriesByBackend)
	prometheus.MustRegister(m.requestsByBackend)
	prometheus.MustRegister(m.responseTimeByBackend)
	prometheus.MustRegister(m.selectionEvents)
	prometheus.MustRegister(m.affinityHits)
	return m
}
