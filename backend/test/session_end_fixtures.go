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

// InstanceStatus reads one activity instance's lifecycle status.
func InstanceStatus(tb testing.TB, db *bun.DB, instanceID int64) string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var status string
	require.NoError(tb, db.NewSelect().TableExpr("schedule.activity_instances").Column("status").Where("id = ?", instanceID).Scan(ctx, &status))
	return status
}

// InstanceStudentByID reloads one instance_students row.
func InstanceStudentByID(tb testing.TB, db *bun.DB, id int64) *schedule.InstanceStudent {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &schedule.InstanceStudent{}
	require.NoError(tb, db.NewSelect().Model(row).ModelTableExpr(`schedule.instance_students AS "instance_student"`).Where(`"instance_student".id = ?`, id).Scan(ctx))
	return row
}
