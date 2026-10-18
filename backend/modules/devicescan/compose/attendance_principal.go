package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// AttendanceTenantID is the school the web attendance request acts for, 0
// when none.
func AttendanceTenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// MarkAttendanceRollback marks the web attendance request's tenant
// transaction for rollback.
func MarkAttendanceRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
