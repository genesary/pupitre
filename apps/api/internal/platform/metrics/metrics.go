// Package metrics defines and exposes the Prometheus metrics collectors
// shared by the pupitre services. Every collector is registered in each
// binary that imports this package, so a service reports zero for the
// gauges it does not own.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// httpEndpointLabel is the label name used to record the request route.
const httpEndpointLabel = "endpoint"

// httpMethodLabel is the label name used to record the HTTP method.
const httpMethodLabel = "method"

// singletons registered once at init time; used by internal/handlers.
var (
	// HTTPRequestsTotal counts HTTP requests by method, endpoint and
	// status.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{ //nolint:gochecknoglobals // promauto collectors must be package-level
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{httpMethodLabel, httpEndpointLabel, "status"})

	// HTTPRequestDuration records HTTP request latency by method and
	// endpoint.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{ //nolint:gochecknoglobals // promauto collectors must be package-level
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	}, []string{httpMethodLabel, httpEndpointLabel})

	// ActiveUsers is a gauge of currently registered users.
	ActiveUsers = promauto.NewGauge(prometheus.GaugeOpts{ //nolint:gochecknoglobals,promlinter // promauto collectors must be package-level; deployed metric name, see dashboards
		Name: "pupitre_active_users_total",
		Help: "Total number of registered users",
	})

	// ActiveCourses is a gauge of currently published courses.
	ActiveCourses = promauto.NewGauge(prometheus.GaugeOpts{ //nolint:gochecknoglobals,promlinter // promauto collectors must be package-level; deployed metric name, see dashboards
		Name: "pupitre_active_courses_total",
		Help: "Total number of published courses",
	})

	// EnrollmentsTotal is a gauge of current course enrollments.
	EnrollmentsTotal = promauto.NewGauge(prometheus.GaugeOpts{ //nolint:gochecknoglobals,promlinter // promauto collectors must be package-level; deployed metric name, see dashboards
		Name: "pupitre_enrollments_total",
		Help: "Total course enrollments",
	})
)

// Handler returns the HTTP handler that serves Prometheus metrics.
func Handler() http.HandlerFunc {
	h := promhttp.Handler()

	return func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }
}
