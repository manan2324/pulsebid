// Package metrics provides per-service Prometheus registries and pool collectors.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Metrics owns its registry; there is no process-global registration.
type Metrics struct {
	Registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	service  string
}

// New registers runtime, process, and HTTP collectors.
func New(service string) *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(), service: service,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_server_requests_total", Help: "Completed HTTP requests."}, []string{"service", "route", "method", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_server_request_duration_seconds", Help: "HTTP request duration in seconds.", Buckets: []float64{.001, .0025, .005, .01, .02, .035, .05, .075, .1, .12, .15, .25, .5, 1}}, []string{"service", "route"}),
	}
	m.Registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), m.requests, m.duration)
	return m
}

// Handler exposes only this service's registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// Middleware observes the mux-populated pattern, never the raw URL or identifiers.
// Register recovery inside this middleware so recovered panics count as HTTP 500.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		measured := httpsnoop.CaptureMetrics(next, w, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		method := r.Method
		switch method {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		default:
			method = "OTHER"
		}
		m.requests.WithLabelValues(m.service, route, method, strconv.Itoa(measured.Code)).Inc()
		m.duration.WithLabelValues(m.service, route).Observe(measured.Duration.Seconds())
		span := trace.SpanFromContext(r.Context())
		span.SetName(route)
		span.SetAttributes(attribute.String("http.route", route))
	})
}

// RegisterPGX observes a pgx pool without making database calls during a scrape.
func (m *Metrics) RegisterPGX(pool *pgxpool.Pool) error {
	return m.Registry.Register(&poolCollector{kind: "db", service: m.service, read: func() (float64, float64) {
		stats := pool.Stat()
		return float64(stats.AcquiredConns()), float64(stats.IdleConns())
	}})
}

// RegisterRedis observes connection-pool usage without network calls.
func (m *Metrics) RegisterRedis(client *redis.Client) error {
	return m.Registry.Register(&poolCollector{kind: "redis", service: m.service, read: func() (float64, float64) {
		stats := client.PoolStats()
		return float64(stats.TotalConns) - float64(stats.IdleConns), float64(stats.IdleConns)
	}})
}

type poolCollector struct {
	kind, service string
	read          func() (float64, float64)
}

func (p *poolCollector) descriptions() (*prometheus.Desc, *prometheus.Desc) {
	labels := prometheus.Labels{"service": p.service}
	return prometheus.NewDesc(p.kind+"_pool_acquired", "Connections currently acquired.", nil, labels), prometheus.NewDesc(p.kind+"_pool_idle", "Connections currently idle.", nil, labels)
}
func (p *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	acquired, idle := p.descriptions()
	ch <- acquired
	ch <- idle
}
func (p *poolCollector) Collect(ch chan<- prometheus.Metric) {
	acquired, idle := p.descriptions()
	a, i := p.read()
	ch <- prometheus.MustNewConstMetric(acquired, prometheus.GaugeValue, a)
	ch <- prometheus.MustNewConstMetric(idle, prometheus.GaugeValue, i)
}
