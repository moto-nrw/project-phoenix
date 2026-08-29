package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/observability"
)

type httpMetrics struct {
	mu             sync.Mutex
	activeRequests int64
	routes         map[routeKey]*routeStats
}

type routeKey struct {
	method      string
	route       string
	statusClass string
}

type routeStats struct {
	count        uint64
	errorCount   uint64
	totalLatency time.Duration
	maxLatency   time.Duration
}

type routeSnapshot struct {
	Method       string `json:"method"`
	Route        string `json:"route"`
	StatusClass  string `json:"status_class"`
	Count        uint64 `json:"count"`
	ErrorCount   uint64 `json:"error_count"`
	AvgLatencyMS int64  `json:"avg_latency_ms"`
	MaxLatencyMS int64  `json:"max_latency_ms"`
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{routes: make(map[routeKey]*routeStats)}
}

func (m *httpMetrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/internal/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		observability.IncActiveHTTPRequests()
		atomic.AddInt64(&m.activeRequests, 1)
		defer func() {
			atomic.AddInt64(&m.activeRequests, -1)
			observability.DecActiveHTTPRequests()
		}()

		next.ServeHTTP(recorder, r)
		duration := time.Since(started)
		m.record(r.Method, apiCommon.RoutePattern(r), recorder.status, duration)
		observability.ObserveHTTPRequest(r.Method, apiCommon.RoutePattern(r), recorder.status, duration)
	})
}

func (m *httpMetrics) record(method, route string, status int, latency time.Duration) {
	key := routeKey{method: method, route: route, statusClass: strconv.Itoa(status/100) + "xx"}
	m.mu.Lock()
	defer m.mu.Unlock()
	stats := m.routes[key]
	if stats == nil {
		stats = &routeStats{}
		m.routes[key] = stats
	}
	stats.count++
	if status >= http.StatusInternalServerError {
		stats.errorCount++
	}
	stats.totalLatency += latency
	if latency > stats.maxLatency {
		stats.maxLatency = latency
	}
}

func (m *httpMetrics) snapshot(limit int) (int64, []routeSnapshot) {
	active := atomic.LoadInt64(&m.activeRequests)
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshots := make([]routeSnapshot, 0, len(m.routes))
	for key, stats := range m.routes {
		average := time.Duration(0)
		if stats.count > 0 {
			average = stats.totalLatency / time.Duration(stats.count)
		}
		snapshots = append(snapshots, routeSnapshot{
			Method: key.method, Route: key.route, StatusClass: key.statusClass,
			Count: stats.count, ErrorCount: stats.errorCount,
			AvgLatencyMS: average.Milliseconds(), MaxLatencyMS: stats.maxLatency.Milliseconds(),
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ErrorCount != snapshots[j].ErrorCount {
			return snapshots[i].ErrorCount > snapshots[j].ErrorCount
		}
		if snapshots[i].MaxLatencyMS != snapshots[j].MaxLatencyMS {
			return snapshots[i].MaxLatencyMS > snapshots[j].MaxLatencyMS
		}
		return snapshots[i].Count > snapshots[j].Count
	})
	if limit > 0 && len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}
	return active, snapshots
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type dbCapacityStats struct {
	openConnections   int
	inUse             int
	idle              int
	waitCount         int64
	waitDuration      time.Duration
	maxIdleClosed     int64
	maxLifetimeClosed int64
}

type sseClientCounter interface{ GetClientCount() int }

type capacityLogger struct {
	dbStats     func() dbCapacityStats
	sse         sseClientCounter
	httpMetrics *httpMetrics
	logger      *slog.Logger
	interval    time.Duration
}

func newCapacityLogger(dbStats func() dbCapacityStats, sse sseClientCounter, metrics *httpMetrics, logger *slog.Logger) *capacityLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &capacityLogger{dbStats: dbStats, sse: sse, httpMetrics: metrics, logger: logger, interval: time.Minute}
}

func (l *capacityLogger) Start(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.LogSnapshot()
		}
	}
}

func (l *capacityLogger) LogSnapshot() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	activeRequests, routes := int64(0), []routeSnapshot(nil)
	if l.httpMetrics != nil {
		activeRequests, routes = l.httpMetrics.snapshot(20)
	}
	stats := dbCapacityStats{}
	if l.dbStats != nil {
		stats = l.dbStats()
	}
	sseClients := 0
	if l.sse != nil {
		sseClients = l.sse.GetClientCount()
	}
	l.logger.Info("capacity snapshot",
		slog.Int("goroutines", runtime.NumGoroutine()),
		slog.Uint64("heap_alloc_bytes", mem.HeapAlloc),
		slog.Uint64("heap_sys_bytes", mem.HeapSys),
		slog.Uint64("total_alloc_bytes", mem.TotalAlloc),
		slog.Int("gc_cycles", int(mem.NumGC)),
		slog.Int64("http_active_requests", activeRequests),
		slog.Any("http_routes", routes),
		slog.Int("sse_clients", sseClients),
		slog.Int("db_open_connections", stats.openConnections),
		slog.Int("db_in_use", stats.inUse),
		slog.Int("db_idle", stats.idle),
		slog.Int64("db_wait_count", stats.waitCount),
		slog.Int64("db_wait_duration_ms", stats.waitDuration.Milliseconds()),
		slog.Int64("db_max_idle_closed", stats.maxIdleClosed),
		slog.Int64("db_max_lifetime_closed", stats.maxLifetimeClosed),
	)
}

func metricsHandler() http.Handler {
	handler := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.RefreshGauges()
		handler.ServeHTTP(w, r)
	})
}

func metricsAuthMiddleware(token string) func(http.Handler) http.Handler {
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
