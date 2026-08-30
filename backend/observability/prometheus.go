package observability

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBStats is the database-capacity snapshot consumed by metrics. It keeps the
// public observability interface independent of database/sql and ORM types.
type DBStats struct {
	OpenConnections   int
	InUse             int
	Idle              int
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64
}

type SSEStats struct {
	ClientsByTenant map[int64]int
}

type SSEStatsProvider interface {
	SnapshotStats() SSEStats
}

// PWAUsageStat is one (tenant, portal) bucket of PWA standalone-usage
// counts (#2189).
type PWAUsageStat struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}

// PWAUsageStatsProvider supplies the standalone-usage counts on scrape.
// Implementations are expected to cache internally because the metrics
// adapter calls this on every scrape.
type PWAUsageStatsProvider interface {
	SnapshotUsageStats() ([]PWAUsageStat, error)
}

// PWAUsageStatsProviderFunc adapts a function to PWAUsageStatsProvider.
type PWAUsageStatsProviderFunc func() ([]PWAUsageStat, error)

// SnapshotUsageStats implements PWAUsageStatsProvider.
func (f PWAUsageStatsProviderFunc) SnapshotUsageStats() ([]PWAUsageStat, error) { return f() }

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
	tenantRuntimeEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_tenant_runtime_events_total",
			Help: "Rejected tenant entry points and tenant transaction failures.",
		},
		[]string{"entry_point", "outcome"},
	)
	unitOfWorkDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_duration_seconds",
			Help:    "Transaction duration by entry point and result.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"entry_point", "result"},
	)
	unitOfWorkRollbacks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_unit_of_work_rollbacks_total",
			Help: "Rolled-back transactions by entry point.",
		},
		[]string{"entry_point"},
	)
	unitOfWorkRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_unit_of_work_retries_total",
			Help: "Deadlock and serialization retries owned by an outer UnitOfWork.",
		},
		[]string{"entry_point"},
	)
	unitOfWorkPoolWait = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_pool_wait_seconds",
			Help:    "Database-pool wait attributed to UnitOfWork execution.",
			Buckets: []float64{0, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"entry_point"},
	)
	unitOfWorkLockWait = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_lock_wait_seconds",
			Help:    "Explicit transaction-lock acquisition time by entry point.",
			Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"entry_point"},
	)
	workerJobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_worker_job_duration_seconds",
			Help:    "Embedded Worker job run duration by stable job ID and outcome.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
		},
		[]string{"job_id", "outcome"},
	)
	settingsLookups = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_settings_lookups_total",
			Help: "Settings lookups by registry key, cache path, and outcome.",
		},
		[]string{"key", "cache", "outcome"},
	)
	settingsLookupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_settings_lookup_duration_seconds",
			Help:    "Settings resolution duration by registry key.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		},
		[]string{"key"},
	)
	settingsSideEffectFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_settings_side_effect_failures_total",
			Help: "Transactional settings side-effect failures by registry key.",
		},
		[]string{"key"},
	)
	rateLimitRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_rate_limit_rejections_total",
			Help: "Requests rejected by the API rate limiter, split by quota bucket.",
		},
		[]string{"bucket"},
	)
	authorizationDenials = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_authorization_denials_total",
			Help: "Authorization denials by stable reason code.",
		},
		[]string{"reason"},
	)
	authMiddlewareDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_auth_middleware_duration_seconds",
			Help:    "Security-principal middleware duration by outcome.",
			Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05},
		},
		[]string{"outcome"},
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
	pwaStandaloneUsers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_pwa_standalone_users",
			Help: "Accounts that used the app in PWA standalone mode within the last 30 days, by tenant and portal.",
		},
		[]string{"tenant_id", "portal"},
	)
	pwaEligibleUsers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_pwa_eligible_users",
			Help: "Accounts with an active mapping matching the portal's role predicate, by tenant and portal.",
		},
		[]string{"tenant_id", "portal"},
	)

	dbStatsMu        sync.RWMutex
	dbStatsProvider  func() DBStats
	sseStatsMu       sync.RWMutex
	sseStatsProvider SSEStatsProvider
	sseGaugeMu       sync.Mutex
	sseGaugeTenants  = make(map[string]struct{})

	pwaStatsMu       sync.RWMutex
	pwaStatsProvider PWAUsageStatsProvider
	pwaGaugeMu       sync.Mutex
	pwaGaugeLabels   = make(map[[2]string]struct{})

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
		tenantRuntimeEvents,
		unitOfWorkDuration,
		unitOfWorkRollbacks,
		unitOfWorkRetries,
		unitOfWorkPoolWait,
		unitOfWorkLockWait,
		workerJobDuration,
		settingsLookups,
		settingsLookupDuration,
		settingsSideEffectFailures,
		rateLimitRejections,
		authorizationDenials,
		authMiddlewareDuration,
		iotRequests,
		iotDuration,
		sseBroadcasts,
		sseDropped,
		sseConnections,
		sseClients,
		pwaStandaloneUsers,
		pwaEligibleUsers,
		dbStatsCollector{},
	)
}

func MetricsBearerTokenFromEnv(getenv func(string) string) (string, error) {
	token := strings.TrimSpace(getenv("METRICS_BEARER_TOKEN"))
	if token == "" {
		return "", errMetricsBearerTokenEmpty
	}
	return token, nil
}

func RegisterDBStatsProvider(provider func() DBStats) {
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()
	dbStatsProvider = provider
}

func RegisterSSEStatsProvider(provider SSEStatsProvider) {
	sseStatsMu.Lock()
	defer sseStatsMu.Unlock()
	sseStatsProvider = provider
}

// RegisterPWAUsageStatsProvider wires the PWA standalone-usage source
// (#2189). The provider runs on every scrape and must cache internally.
func RegisterPWAUsageStatsProvider(provider PWAUsageStatsProvider) {
	pwaStatsMu.Lock()
	defer pwaStatsMu.Unlock()
	pwaStatsProvider = provider
}

// RefreshGauges updates scrape-time gauges before the HTTP adapter serves the
// Prometheus registry.
func RefreshGauges() {
	refreshSSEGauges()
	refreshPWAGauges()
}

func IncActiveHTTPRequests() {
	appHTTPActive.Inc()
}

func DecActiveHTTPRequests() {
	appHTTPActive.Dec()
}

func ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	method = normalizeHTTPMethod(method)
	statusClass := StatusClass(status)
	appHTTPRequests.WithLabelValues(method, route, statusClass).Inc()
	appHTTPDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

func ObserveTenantRequest(tenantID int64, scope, method, route string, status int, duration time.Duration, txOutcome string) {
	method = normalizeHTTPMethod(method)
	tenant := strconv.FormatInt(tenantID, 10)
	statusClass := StatusClass(status)
	tenantHTTPRequests.WithLabelValues(tenant, scope, method, route, statusClass, txOutcome).Inc()
	tenantHTTPDuration.WithLabelValues(tenant, scope, method, route).Observe(duration.Seconds())
}

func normalizeHTTPMethod(method string) string {
	switch strings.ToUpper(method) {
	case "CONNECT", "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "TRACE":
		return strings.ToUpper(method)
	default:
		return "other"
	}
}

func RecordTenantRuntimeEvent(entryPoint, outcome string) {
	tenantRuntimeEvents.WithLabelValues(sanitizeLabel(entryPoint), sanitizeLabel(outcome)).Inc()
}

func RecordUnitOfWorkEvent(entryPoint, kind, result string, duration time.Duration, retries int) {
	entryPoint = sanitizeLabel(entryPoint)
	switch kind {
	case "transaction":
		result = sanitizeLabel(result)
		unitOfWorkDuration.WithLabelValues(entryPoint, result).Observe(duration.Seconds())
		if result == "rollback" || result == "panic" {
			unitOfWorkRollbacks.WithLabelValues(entryPoint).Inc()
		}
		if retries > 0 {
			unitOfWorkRetries.WithLabelValues(entryPoint).Add(float64(retries))
		}
	case "pool_wait":
		unitOfWorkPoolWait.WithLabelValues(entryPoint).Observe(duration.Seconds())
	case "lock_wait":
		unitOfWorkLockWait.WithLabelValues(entryPoint).Observe(duration.Seconds())
	}
}

// RecordWorkerRunEvent records one bounded embedded-job outcome.
func RecordWorkerRunEvent(jobID, outcome string, duration time.Duration) {
	workerJobDuration.WithLabelValues(sanitizeLabel(jobID), sanitizeLabel(outcome)).Observe(duration.Seconds())
}

func ObserveSettingsLookup(key, cache, outcome string, duration time.Duration) {
	settingsLookups.WithLabelValues(sanitizeLabel(key), sanitizeLabel(cache), sanitizeLabel(outcome)).Inc()
	settingsLookupDuration.WithLabelValues(sanitizeLabel(key)).Observe(duration.Seconds())
}

func RecordSettingsSideEffectFailure(key string) {
	settingsSideEffectFailures.WithLabelValues(sanitizeLabel(key)).Inc()
}

func RecordRateLimitRejection(bucket string) {
	rateLimitRejections.WithLabelValues(sanitizeLabel(bucket)).Inc()
}

func RecordAuthorizationEvent(outcome, reason string, duration time.Duration) {
	switch reason {
	case "invalid_principal", "missing_principal", "permission_denied":
		authorizationDenials.WithLabelValues(reason).Inc()
	case "":
	default:
		authorizationDenials.WithLabelValues("unknown").Inc()
	}
	if outcome == "resolved" || outcome == "invalid" {
		authMiddlewareDuration.WithLabelValues(outcome).Observe(duration.Seconds())
	}
}

func ObserveIoTRequest(tenantID int64, method, route string, status int, duration time.Duration, deviceType string) {
	method = normalizeHTTPMethod(method)
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

func StatusClass(status int) string {
	if status <= 0 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
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
	case status == 401 || status == 403:
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
	stats := provider()
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

// refreshPWAGauges pulls the standalone-usage counts on scrape, zeroing
// label pairs that disappeared. A provider error keeps the previous values
// — a failed refresh must not turn into a fake zero.
func refreshPWAGauges() {
	pwaStatsMu.RLock()
	provider := pwaStatsProvider
	pwaStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats, err := provider.SnapshotUsageStats()
	if err != nil {
		return
	}
	current := make(map[[2]string]struct{}, len(stats))
	pwaGaugeMu.Lock()
	defer pwaGaugeMu.Unlock()
	for _, stat := range stats {
		labels := [2]string{strconv.FormatInt(stat.TenantID, 10), stat.Portal}
		current[labels] = struct{}{}
		pwaStandaloneUsers.WithLabelValues(labels[0], labels[1]).Set(float64(stat.StandaloneUsers))
		pwaEligibleUsers.WithLabelValues(labels[0], labels[1]).Set(float64(stat.EligibleUsers))
	}
	for labels := range pwaGaugeLabels {
		if _, ok := current[labels]; !ok {
			pwaStandaloneUsers.WithLabelValues(labels[0], labels[1]).Set(0)
			pwaEligibleUsers.WithLabelValues(labels[0], labels[1]).Set(0)
		}
	}
	pwaGaugeLabels = current
}
