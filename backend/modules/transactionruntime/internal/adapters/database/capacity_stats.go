package database

import (
	"time"

	"github.com/uptrace/bun"
)

// CapacityStats is a scalar snapshot of the SQL connection pool.
type CapacityStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
}

// SnapshotCapacity reads connection-pool statistics inside the database
// adapter so composition roots do not depend on database/sql contracts.
func SnapshotCapacity(db *bun.DB) CapacityStats {
	stats := db.Stats()
	return CapacityStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}
}
