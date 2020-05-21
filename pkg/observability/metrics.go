package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var DefaultMetrics = NewMetrics(prometheus.DefaultRegisterer)

func RegisterMetrics(mux *http.ServeMux, reg prometheus.Gatherer) {
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
}

type Metrics struct {
	GRPCRunning             prometheus.Gauge
	ServiceCount            prometheus.Gauge
	BackendsCount           *prometheus.GaugeVec
	CanariesCount           prometheus.Gauge
	EventsCount             *prometheus.CounterVec
	RenewServiceErrorsCount *prometheus.CounterVec
	HandlerLatency          *prometheus.HistogramVec
	ResourceVersion         *prometheus.GaugeVec
	RejectedResources       *prometheus.CounterVec

	registry prometheus.Registerer
}

func NewMetrics(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		GRPCRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "navigator_grpc_running",
			Help: "Navigator start to serve envoy grpc requests",
		}),
		CanariesCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "navigator_canary_cache_canaries_count",
			Help: "Count of canaried services",
		}),
		ServiceCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "navigator_k8s_cache_services_count",
			Help: "Count of services, stored in k8s cache",
		}),
		BackendsCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "navigator_k8s_cache_backends_count",
				Help: "Count of backends by service, stored in k8s cache",
			},
			[]string{"service", "clusterID"},
		),
		EventsCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "navigator_informer_events_total",
				Help: "Current amount of k8s events, received by informers",
			},
			[]string{"action", "informer", "clusterID"},
		),
		RenewServiceErrorsCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "navigator_canary_service_renew_errors_total",
				Help: "Current amount of errors during canary service renewal",
			},
			[]string{"clusterID"},
		),
		HandlerLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "navigator_kube_handler_latency_seconds",
				Help: "K8s handlers latency",
			},
			[]string{"informer"},
		),
		RejectedResources: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "navigator_rejected_envoy_resources_total",
				Help: "Current cached envoy resource version",
			},
			[]string{"type_url", "resource_name", "app", "con_id", "cluster"},
		),
		registry: registry,
	}
	m.registry.MustRegister(
		m.GRPCRunning,
		m.ServiceCount,
		m.BackendsCount,
		m.CanariesCount,
		m.EventsCount,
		m.RenewServiceErrorsCount,
		m.HandlerLatency,
		m.RejectedResources,
	)
	return m
}

func (m *Metrics) MustRegister(collectors ...prometheus.Collector) {
	m.registry.MustRegister(collectors...)
}

func (m *Metrics) Reset() {
	m.GRPCRunning.Set(0)
	m.ServiceCount.Set(0)
	m.BackendsCount.Reset()
	m.CanariesCount.Set(0)
	m.EventsCount.Reset()
	m.RenewServiceErrorsCount.Reset()
	m.HandlerLatency.Reset()
	m.RejectedResources.Reset()
}
