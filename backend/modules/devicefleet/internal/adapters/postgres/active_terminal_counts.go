package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// countActiveTerminalsByTenantSQL is the billing rule for an active terminal
// (#2791): live, not virtual, status active.
const countActiveTerminalsByTenantSQL = `SELECT tenant_id, COUNT(*) AS count
FROM iot.devices
WHERE archived_at IS NULL AND device_type <> 'virtual' AND status = 'active'
GROUP BY tenant_id`

// CountActiveTerminalsByTenant counts the active terminals of every school
// the connection can see.
func CountActiveTerminalsByTenant(ctx context.Context, db bun.IDB) (map[int64]int, error) {
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	if err := db.NewRaw(countActiveTerminalsByTenantSQL).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("devicefleet postgres: count active terminals by tenant: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.TenantID] = row.Count
	}
	return counts, nil
}
