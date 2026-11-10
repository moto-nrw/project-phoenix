package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBStats is the database-capacity snapshot consumed by metrics. It keeps the
// public observability interface independent of database/sql and ORM types.
type DBStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
}

var (
	dbStatsMu       sync.RWMutex
	dbStatsProvider func() DBStats

	dbMaxOpenConnectionsDesc = prometheus.NewDesc("phoenix_db_max_open_connections", "Configured upper limit of open DB connections in the pool (DB_MAX_OPEN_CONNS).", nil, nil)
	dbOpenConnectionsDesc    = prometheus.NewDesc("phoenix_db_open_connections", "Open DB connections.", nil, nil)
	dbInUseConnectionsDesc   = prometheus.NewDesc("phoenix_db_in_use_connections", "DB connections currently in use.", nil, nil)
	dbIdleConnectionsDesc    = prometheus.NewDesc("phoenix_db_idle_connections", "Idle DB connections.", nil, nil)
	dbWaitCountDesc          = prometheus.NewDesc("phoenix_db_wait_count_total", "Total waits for a DB connection.", nil, nil)
	dbWaitDurationDesc       = prometheus.NewDesc("phoenix_db_wait_duration_seconds_total", "Total wait time for DB connections.", nil, nil)
	dbMaxIdleClosedDesc      = prometheus.NewDesc("phoenix_db_max_idle_closed_total", "DB connections closed due to max idle.", nil, nil)
	dbMaxLifetimeClosedDesc  = prometheus.NewDesc("phoenix_db_max_lifetime_closed_total", "DB connections closed due to max lifetime.", nil, nil)
)

func RegisterDBStatsProvider(provider func() DBStats) {
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()
	dbStatsProvider = provider
}

type dbStatsCollector struct{}

func (dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbMaxOpenConnectionsDesc
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
	emitDBGauge(ch, dbMaxOpenConnectionsDesc, float64(stats.MaxOpenConnections))
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
