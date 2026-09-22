package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/uptrace/bun"
)

// countActiveStudentsByTenantSQL is the billing rule for "actively managed":
// a live membership in status active whose enrollment interval includes the
// capture day (#2791).
const countActiveStudentsByTenantSQL = `SELECT tenant_id, COUNT(*) AS count
FROM users.student_school_memberships
WHERE deleted_at IS NULL
  AND status = 'active'
  AND (enrolled_from IS NULL OR enrolled_from <= ?::date)
  AND (enrolled_until IS NULL OR enrolled_until >= ?::date)
GROUP BY tenant_id`

// CountActiveStudentsByTenant counts the active students of every school the
// connection can see.
func CountActiveStudentsByTenant(ctx context.Context, db bun.IDB, capturedAt time.Time) (map[int64]int, error) {
	captureDate := calendar.DateFromTime(capturedAt)
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	if err := db.NewRaw(countActiveStudentsByTenantSQL, captureDate, captureDate).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: count active students by tenant: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.TenantID] = row.Count
	}
	return counts, nil
}
