package observability

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type SSEStats struct {
	ClientsByTenant map[int64]int
}

type SSEStatsProvider interface {
	SnapshotStats() SSEStats
}

var (
	appHTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_backend_http_requests_total",
			Help: "Backend HTTP requests by route and status class.",
		},
		[]string{"method", "route", "status_class"},
	)
	appHTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_backend_http_request_duration_seconds",
			Help:    "Backend HTTP request duration by route.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)
	appHTTPActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "phoenix_backend_http_active_requests",
			Help: "Currently active backend HTTP requests.",
		},
	)
	tenantHTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_tenant_http_requests_total",
			Help: "Tenant-scoped backend requests by tenant, route, and status class.",
		},
		[]string{"tenant_id", "scope", "method", "route", "status_class", "tx_outcome"},
	)
	tenantHTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_tenant_http_request_duration_seconds",
			Help:    "Tenant-scoped backend request duration.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"tenant_id", "scope", "method", "route"},
	)
	iotRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_iot_requests_total",
			Help: "IoT requests by tenant, route, device type, and outcome.",
		},
		[]string{"tenant_id", "method", "route", "status_class", "device_type", "outcome"},
	)
	iotDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_iot_request_duration_seconds",
			Help:    "IoT request duration by tenant, route, and device type.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"tenant_id", "method", "route", "device_type"},
	)
	sseBroadcasts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_broadcasts_total",
			Help: "SSE broadcasts by tenant, event type, and target.",
		},
		[]string{"tenant_id", "event_type", "target"},
	)
	sseDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_dropped_events_total",
			Help: "SSE events dropped because a client channel was full.",
		},
		[]string{"tenant_id", "event_type", "target"},
	)
	sseConnections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_connections_total",
			Help: "SSE connection lifecycle events.",
		},
		[]string{"tenant_id", "event"},
	)
	sseClients = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_sse_clients",
			Help: "Currently connected SSE clients by tenant.",
		},
		[]string{"tenant_id"},
	)

	// Email delivery webhook metrics (#1937). Deliberately carry no
	// tenant_id label: the webhook is unauthenticated, so any label derived
	// from the request before the outbox row is resolved would be
	// attacker-controlled cardinality.
	emailWebhookRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_email_webhook_requests_total",
			Help: "Email delivery webhook requests by verification outcome.",
		},
		[]string{"provider", "result"},
	)
	emailDeliveryEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_email_delivery_events_total",
			Help: "Ingested email delivery events by type and ingest outcome.",
		},
		[]string{"provider", "event_type", "result"},
	)
	emailDeliveryStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_email_delivery_status_total",
			Help: "Applied email delivery-status transitions by resulting status.",
		},
		[]string{"status"},
	)

	dbStatsMu        sync.RWMutex
	dbStatsProvider  DBStatsProvider
	sseStatsMu       sync.RWMutex
	sseStatsProvider SSEStatsProvider
	sseGaugeMu       sync.Mutex
	sseGaugeTenants  = make(map[string]struct{})

	dbOpenConnectionsDesc      = prometheus.NewDesc("phoenix_db_open_connections", "Open DB connections.", nil, nil)
	dbInUseConnectionsDesc     = prometheus.NewDesc("phoenix_db_in_use_connections", "DB connections currently in use.", nil, nil)
	dbIdleConnectionsDesc      = prometheus.NewDesc("phoenix_db_idle_connections", "Idle DB connections.", nil, nil)
	dbWaitCountDesc            = prometheus.NewDesc("phoenix_db_wait_count_total", "Total waits for a DB connection.", nil, nil)
	dbWaitDurationDesc         = prometheus.NewDesc("phoenix_db_wait_duration_seconds_total", "Total wait time for DB connections.", nil, nil)
	dbMaxIdleClosedDesc        = prometheus.NewDesc("phoenix_db_max_idle_closed_total", "DB connections closed due to max idle.", nil, nil)
	dbMaxLifetimeClosedDesc    = prometheus.NewDesc("phoenix_db_max_lifetime_closed_total", "DB connections closed due to max lifetime.", nil, nil)
	errMetricsBearerTokenEmpty = errors.New("METRICS_BEARER_TOKEN is required")
)

func init() {
	prometheus.MustRegister(
		appHTTPRequests,
		appHTTPDuration,
		appHTTPActive,
		tenantHTTPRequests,
		tenantHTTPDuration,
		iotRequests,
		iotDuration,
		sseBroadcasts,
		sseDropped,
		sseConnections,
		sseClients,
		emailWebhookRequests,
		emailDeliveryEvents,
		emailDeliveryStatus,
		dbStatsCollector{},
	)
}

func MetricsHandler() http.Handler {
	handler := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshSSEGauges()
		handler.ServeHTTP(w, r)
	})
}

func MetricsBearerTokenFromEnv(getenv func(string) string) (string, error) {
	token := strings.TrimSpace(getenv("METRICS_BEARER_TOKEN"))
	if token == "" {
		return "", errMetricsBearerTokenEmpty
	}
	return token, nil
}

func MetricsAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RegisterDBStatsProvider(provider DBStatsProvider) {
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()
	dbStatsProvider = provider
}

func RegisterSSEStatsProvider(provider SSEStatsProvider) {
	sseStatsMu.Lock()
	defer sseStatsMu.Unlock()
	sseStatsProvider = provider
}

func IncActiveHTTPRequests() {
	appHTTPActive.Inc()
}

func DecActiveHTTPRequests() {
	appHTTPActive.Dec()
}

func ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	statusClass := StatusClass(status)
	appHTTPRequests.WithLabelValues(method, route, statusClass).Inc()
	appHTTPDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

func ObserveTenantRequest(tenantID int64, scope, method, route string, status int, duration time.Duration, txOutcome string) {
	tenant := strconv.FormatInt(tenantID, 10)
	statusClass := StatusClass(status)
	tenantHTTPRequests.WithLabelValues(tenant, scope, method, route, statusClass, txOutcome).Inc()
	tenantHTTPDuration.WithLabelValues(tenant, scope, method, route).Observe(duration.Seconds())
}

func ObserveIoTRequest(tenantID int64, method, route string, status int, duration time.Duration, deviceType string) {
	tenant := strconv.FormatInt(tenantID, 10)
	if tenantID <= 0 {
		tenant = "unknown"
	}
	if deviceType == "" {
		deviceType = "unknown"
	}
	statusClass := StatusClass(status)
	iotRequests.WithLabelValues(tenant, method, route, statusClass, sanitizeLabel(deviceType), outcomeForStatus(status)).Inc()
	iotDuration.WithLabelValues(tenant, method, route, sanitizeLabel(deviceType)).Observe(duration.Seconds())
}

func RecordSSEConnection(tenantID int64, event string) {
	sseConnections.WithLabelValues(strconv.FormatInt(tenantID, 10), event).Inc()
}

func RecordSSEBroadcast(tenantID int64, eventType, target string, dropped int) {
	tenant := strconv.FormatInt(tenantID, 10)
	if tenantID == 0 {
		tenant = "all"
	}
	sseBroadcasts.WithLabelValues(tenant, sanitizeLabel(eventType), sanitizeLabel(target)).Inc()
	if dropped > 0 {
		sseDropped.WithLabelValues(tenant, sanitizeLabel(eventType), sanitizeLabel(target)).Add(float64(dropped))
	}
}

// RecordEmailWebhook counts one webhook request by verification outcome.
// Results in use: accepted, invalid_signature, stale_timestamp, malformed,
// not_configured. A non-zero invalid_signature rate is an alertable event —
// nothing legitimate produces one.
func RecordEmailWebhook(provider, result string) {
	emailWebhookRequests.WithLabelValues(sanitizeLabel(provider), sanitizeLabel(result)).Inc()
}

// RecordEmailDeliveryEvent counts one ingested event. A rising unmatched rate
// means correlation is broken (a relay rewriting Message-IDs, or events for a
// different account on a shared provider).
func RecordEmailDeliveryEvent(provider, eventType, result string) {
	emailDeliveryEvents.WithLabelValues(sanitizeLabel(provider), sanitizeLabel(eventType), sanitizeLabel(result)).Inc()
}

// RecordEmailDeliveryStatus counts one applied delivery-status transition.
func RecordEmailDeliveryStatus(status string) {
	emailDeliveryStatus.WithLabelValues(sanitizeLabel(status)).Inc()
}

func StatusClass(status int) string {
	if status <= 0 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

func RoutePattern(r *http.Request) string {
	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if pattern := routeCtx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}

func sanitizeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 64 {
		return value[:64]
	}
	return value
}

func outcomeForStatus(status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "auth_error"
	case status >= 400:
		return "validation_error"
	case status >= 200 && status < 300:
		return "success"
	default:
		return "other"
	}
}

type dbStatsCollector struct{}

func (dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbOpenConnectionsDesc
	ch <- dbInUseConnectionsDesc
	ch <- dbIdleConnectionsDesc
	ch <- dbWaitCountDesc
	ch <- dbWaitDurationDesc
	ch <- dbMaxIdleClosedDesc
	ch <- dbMaxLifetimeClosedDesc
}

func (dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	dbStatsMu.RLock()
	provider := dbStatsProvider
	dbStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats := provider.Stats()
	emitDBGauge(ch, dbOpenConnectionsDesc, float64(stats.OpenConnections))
	emitDBGauge(ch, dbInUseConnectionsDesc, float64(stats.InUse))
	emitDBGauge(ch, dbIdleConnectionsDesc, float64(stats.Idle))
	emitDBGauge(ch, dbWaitCountDesc, float64(stats.WaitCount))
	emitDBGauge(ch, dbWaitDurationDesc, stats.WaitDuration.Seconds())
	emitDBGauge(ch, dbMaxIdleClosedDesc, float64(stats.MaxIdleClosed))
	emitDBGauge(ch, dbMaxLifetimeClosedDesc, float64(stats.MaxLifetimeClosed))
}

func emitDBGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
}

func refreshSSEGauges() {
	sseStatsMu.RLock()
	provider := sseStatsProvider
	sseStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats := provider.SnapshotStats()
	currentTenants := make(map[string]struct{}, len(stats.ClientsByTenant))
	sseGaugeMu.Lock()
	defer sseGaugeMu.Unlock()
	for tenantID, count := range stats.ClientsByTenant {
		tenant := strconv.FormatInt(tenantID, 10)
		currentTenants[tenant] = struct{}{}
		sseClients.WithLabelValues(tenant).Set(float64(count))
	}

	for tenant := range sseGaugeTenants {
		if _, ok := currentTenants[tenant]; !ok {
			sseClients.WithLabelValues(tenant).Set(0)
		}
	}
	sseGaugeTenants = currentTenants
	if sseGaugeTenants == nil {
		sseGaugeTenants = make(map[string]struct{})
	}
}

var _ DBStatsProvider = (*sql.DB)(nil)
