package behaviortest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	cleanuprunner "github.com/moto-ogs/project-phoenix/modules/schedulerruntime/compose/behaviortest"
	"github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type auditedFeedback struct{ school int64 }

func (c auditedFeedback) DeleteExpired(ctx context.Context) (int, error) {
	if testpkg.TenantIDFromContext(ctx) != c.school {
		return 0, nil
	}
	return 2, nil
}

func TestNightlyCleanupWritesOneAuditRowPerType(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 14, 2, 0, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 30)
	deps.FeedbackCleaner = auditedFeedback{school: school.id}
	deps.PWAUsageCleanup = func(ctx context.Context) (ports.PWAUsageCleanupResult, error) {
		result := ports.PWAUsageCleanupResult{RetentionDays: 90}
		if testpkg.TenantIDFromContext(ctx) == school.id {
			result.RowsDeleted = 3
		}
		return result, nil
	}
	deps.StaffMessageCleanup = func(ctx context.Context) (ports.StaffMessageCleanupResult, error) {
		result := ports.StaffMessageCleanupResult{RetentionDays: 365}
		if testpkg.TenantIDFromContext(ctx) == school.id {
			result.MessagesDeleted, result.ThreadsDeleted = 4, 1
		}
		return result, nil
	}
	cleanuprunner.NewCleanupTestRunner(deps)()
	for kind, want := range map[string]int{"feedback_retention": 2, "pwa_usage_retention": 3, "staff_message_retention": 5} {
		var counts []int
		require.NoError(t, admin.NewRaw(`SELECT records_deleted FROM audit.data_deletions WHERE tenant_id = ? AND deletion_type = ? ORDER BY id`, school.id, kind).Scan(t.Context(), &counts))
		require.Equal(t, []int{want}, counts, kind)
	}
}

func TestNightlyCleanupAuditFailureDoesNotRecordSuccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 14, 2, 0, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 30)
	deps.FeedbackCleaner = auditedFeedback{school: school.id}
	originalLog := deps.RetentionLog
	deps.RetentionLog = func(ctx context.Context, kind string, outcome ports.RetentionOutcome) error {
		if kind == "feedback_retention" {
			return errors.New("audit unavailable")
		}
		return originalLog(ctx, kind, outcome)
	}
	cleanuprunner.NewCleanupTestRunner(deps)()
	require.Zero(t, cleanupRunCount(t, admin, school.id), "an unaudited deletion is not a successful nightly run")
	require.Equal(t, 1, storedRun(t, admin, school.id).Failures)
	var auditRows int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM audit.data_deletions WHERE tenant_id = ? AND deletion_type = 'feedback_retention'`, school.id).Scan(t.Context(), &auditRows))
	require.Zero(t, auditRows)
}
