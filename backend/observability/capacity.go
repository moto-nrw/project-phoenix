package observability

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type DBStatsProvider interface {
	Stats() sql.DBStats
}

type SSEClientCounter interface {
	GetClientCount() int
}

type HTTPMetrics struct {
	mu             sync.Mutex
	activeRequests int64
	routes         map[routeKey]*routeStats
}

type routeKey struct {
	Method      string
	Route       string
	StatusClass string
}

type routeStats struct {
	Count        uint64
	ErrorCount   uint64
	TotalLatency time.Duration
	MaxLatency   time.Duration
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

func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		routes: make(map[routeKey]*routeStats),
	}
}

func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/internal/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		IncActiveHTTPRequests()
		atomic.AddInt64(&m.activeRequests, 1)
		defer func() {
			atomic.AddInt64(&m.activeRequests, -1)
			DecActiveHTTPRequests()
		}()

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		m.record(r, recorder.status, duration)
		ObserveHTTPRequest(r.Method, RoutePattern(r), recorder.status, duration)
	})
}

func (m *HTTPMetrics) record(r *http.Request, status int, latency time.Duration) {
	route := RoutePattern(r)

	key := routeKey{
		Method:      r.Method,
		Route:       route,
		StatusClass: strconv.Itoa(status/100) + "xx",
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.routes[key]
	if stats == nil {
		stats = &routeStats{}
		m.routes[key] = stats
	}
	stats.Count++
	if status >= http.StatusInternalServerError {
		stats.ErrorCount++
	}
	stats.TotalLatency += latency
	if latency > stats.MaxLatency {
		stats.MaxLatency = latency
	}
}

func (m *HTTPMetrics) Snapshot(limit int) (int64, []routeSnapshot) {
	active := atomic.LoadInt64(&m.activeRequests)

	m.mu.Lock()
	defer m.mu.Unlock()

	snapshots := make([]routeSnapshot, 0, len(m.routes))
	for key, stats := range m.routes {
		avg := time.Duration(0)
		if stats.Count > 0 {
			avg = stats.TotalLatency / time.Duration(stats.Count)
		}
		snapshots = append(snapshots, routeSnapshot{
			Method:       key.Method,
			Route:        key.Route,
			StatusClass:  key.StatusClass,
			Count:        stats.Count,
			ErrorCount:   stats.ErrorCount,
			AvgLatencyMS: avg.Milliseconds(),
			MaxLatencyMS: stats.MaxLatency.Milliseconds(),
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

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type CapacityLogger struct {
	db          DBStatsProvider
	sse         SSEClientCounter
	httpMetrics *HTTPMetrics
	logger      *slog.Logger
	interval    time.Duration
}

func NewCapacityLogger(db DBStatsProvider, sse SSEClientCounter, httpMetrics *HTTPMetrics, logger *slog.Logger) *CapacityLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &CapacityLogger{
		db:          db,
		sse:         sse,
		httpMetrics: httpMetrics,
		logger:      logger,
		interval:    time.Minute,
	}
}

func (l *CapacityLogger) Start(ctx context.Context) {
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

func (l *CapacityLogger) LogSnapshot() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	activeRequests, routes := int64(0), []routeSnapshot(nil)
	if l.httpMetrics != nil {
		activeRequests, routes = l.httpMetrics.Snapshot(20)
	}

	dbStats := sql.DBStats{}
	if l.db != nil {
		dbStats = l.db.Stats()
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
		slog.Int("db_open_connections", dbStats.OpenConnections),
		slog.Int("db_in_use", dbStats.InUse),
		slog.Int("db_idle", dbStats.Idle),
		slog.Int64("db_wait_count", dbStats.WaitCount),
		slog.Int64("db_wait_duration_ms", dbStats.WaitDuration.Milliseconds()),
		slog.Int64("db_max_idle_closed", dbStats.MaxIdleClosed),
		slog.Int64("db_max_lifetime_closed", dbStats.MaxLifetimeClosed),
	)
}
