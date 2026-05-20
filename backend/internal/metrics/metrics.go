// Package metrics exposes Prometheus metrics for the dashboard.
//
// Two metrics ship by default:
//
//   - http_requests_total{method,path,status}
//     Counter, incremented once per request.
//
//   - http_request_duration_seconds{method,path}
//     Histogram, sampled per request.
//
// "path" is the gin route TEMPLATE (e.g. "/api/admin/users/:id"),
// not the concrete URL — without that, every request to a different
// user id would explode label cardinality and OOM the scraper.
//
// Metrics are registered on package init() so importing the package
// is enough; the operator wires the /metrics handler in app.go.
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests handled, labeled by method, route template, and status code.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, labeled by method and route template.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// Middleware records the request count + duration. It uses
// c.FullPath() instead of c.Request.URL.Path so route params are
// collapsed into the template (e.g. /api/admin/users/:id), which
// keeps cardinality bounded.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			// Unmatched (404) routes — bucket them all together so we
			// don't burn cardinality on bot scans like /wp-login.php.
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// Handler returns the promhttp scrape handler. Wrap in gin.WrapH
// when registering.
func Handler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}

// reset clears the metrics — only used in tests so they don't see
// state from earlier cases. Not exported because production code
// must never zero its own metrics.
func reset() {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()
}
