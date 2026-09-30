package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// VisitExitTime reads one visit's exit time outside any tenant scope so a
// session-end test can verify the close without going through the owner it
// exercises.
func VisitExitTime(tb testing.TB, db *bun.DB, visitID int64) *time.Time {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var exit *time.Time
	require.NoError(tb, db.NewSelect().TableExpr("active.visits").Column("exit_time").Where("id = ?", visitID).Scan(ctx, &exit))
	return exit
}

// InstanceStatus reads one activity instance's lifecycle status: the status
// of its Student Presence session, or the planning status without one.
func InstanceStatus(tb testing.TB, db *bun.DB, instanceID int64) string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var status string
	require.NoError(tb, db.NewRaw(`SELECT coalesce(session.status, instance.status)
		FROM schedule.activity_instances AS instance
		LEFT JOIN active.activity_sessions AS session
			ON session.tenant_id = instance.tenant_id AND session.schedule_instance_id = instance.id
		WHERE instance.id = ?`, instanceID).Scan(ctx, &status))
	return status
}

// InstanceStudentByID reloads one participant with its attendance. A
// participant without an attendance row reads as expected attendance.
func InstanceStudentByID(tb testing.TB, db *bun.DB, id int64) *schedule.InstanceStudent {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return InstanceStudentByIDContext(tb, ctx, db, id)
}
