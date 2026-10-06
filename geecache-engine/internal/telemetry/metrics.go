// Package telemetry owns per-process metrics; core packages only use Observer.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type Metrics struct {
	Registry                                            *prometheus.Registry
	api, peer, source                                   *prometheus.CounterVec
	local                                               *prometheus.CounterVec
	fallback                                            *prometheus.CounterVec
	apiDuration, peerDuration, sourceDuration, slotWait *prometheus.HistogramVec
	bloom, updates, discoveryErrors                     prometheus.Counter
	inflight, members                                   prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry()}
	counter := func(name string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: name}, labels)
	}
	histogram := func(name string) *prometheus.HistogramVec {
		return prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: name, Buckets: prometheus.DefBuckets}, []string{"result"})
	}
	m.api = counter("simplecache_api_requests_total", "result")
	m.peer = counter("simplecache_peer_requests_total", "result")
	m.source = counter("simplecache_source_loads_total", "result")
	m.local = counter("simplecache_local_lookups_total", "kind", "result")
	m.fallback = counter("simplecache_fallback_total", "reason")
	m.apiDuration = histogram("simplecache_api_duration_seconds")
	m.peerDuration = histogram("simplecache_peer_duration_seconds")
	m.sourceDuration = histogram("simplecache_source_duration_seconds")
	m.slotWait = histogram("simplecache_source_slot_wait_seconds")
	m.bloom = prometheus.NewCounter(prometheus.CounterOpts{Name: "simplecache_bloom_rejections_total", Help: "Bloom definite negatives"})
	m.updates = prometheus.NewCounter(prometheus.CounterOpts{Name: "simplecache_membership_updates_total", Help: "Changed membership snapshots"})
	m.discoveryErrors = prometheus.NewCounter(prometheus.CounterOpts{Name: "simplecache_discovery_errors_total", Help: "Discovery list/watch failures"})
	m.inflight = prometheus.NewGauge(prometheus.GaugeOpts{Name: "simplecache_source_inflight", Help: "Executing source getters"})
	m.members = prometheus.NewGauge(prometheus.GaugeOpts{Name: "simplecache_members", Help: "Published member count"})
	m.Registry.MustRegister(m.api, m.peer, m.source, m.local, m.fallback, m.apiDuration, m.peerDuration, m.sourceDuration, m.slotWait, m.bloom, m.updates, m.discoveryErrors, m.inflight, m.members, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}
func (m *Metrics) Observe(operation, result string, elapsed time.Duration) {
	switch operation {
	case "api":
		m.api.WithLabelValues(result).Inc()
		m.apiDuration.WithLabelValues(result).Observe(elapsed.Seconds())
	case "peer":
		m.peer.WithLabelValues(result).Inc()
		m.peerDuration.WithLabelValues(result).Observe(elapsed.Seconds())
	case "source":
		m.source.WithLabelValues(result).Inc()
		m.sourceDuration.WithLabelValues(result).Observe(elapsed.Seconds())
	case "lookup_api":
		m.local.WithLabelValues("api", result).Inc()
	case "lookup_peer":
		m.local.WithLabelValues("peer", result).Inc()
	case "fallback":
		m.fallback.WithLabelValues(result).Inc()
	case "slot":
		m.slotWait.WithLabelValues(result).Observe(elapsed.Seconds())
	case "bloom":
		m.bloom.Inc()
	}
}
func (m *Metrics) Inflight(delta float64) { m.inflight.Add(delta) }
func (m *Metrics) Membership(count int, changed bool) {
	m.members.Set(float64(count))
	if changed {
		m.updates.Inc()
	}
}
func (m *Metrics) DiscoveryError() { m.discoveryErrors.Inc() }
